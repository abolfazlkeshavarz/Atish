// End-to-end smoke test. Run against a server started with DEV_AUTH=true and ADMIN_PASSWORD set:
//   BASE=http://localhost:8080 ADMIN_PASSWORD=... node scripts/smoke.mjs
import zlib from 'node:zlib'

const BASE = process.env.BASE || 'http://localhost:8080'
const ADMIN_PASSWORD = process.env.ADMIN_PASSWORD || 'supersecret-admin-pw'
let failed = 0

const ok = (cond, msg) => {
  if (cond) console.log('  ✓', msg)
  else { failed++; console.log('  ✗ FAIL:', msg) }
}

async function call(method, path, { token, body, form } = {}) {
  const headers = {}
  if (token) headers.Authorization = 'Bearer ' + token
  let payload
  if (form) payload = form
  else if (body !== undefined) { headers['Content-Type'] = 'application/json'; payload = JSON.stringify(body) }
  const res = await fetch(BASE + path, { method, headers, body: payload })
  const text = await res.text()
  let json = null
  try { json = text ? JSON.parse(text) : null } catch { /* binary */ }
  return { status: res.status, json, text }
}

// solid-colour PNG generator (no deps)
function png(w, h, [r, g, b]) {
  const crcT = new Int32Array(256).map((_, n) => { let c = n; for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1; return c })
  const crc = (buf) => { let c = -1; for (const x of buf) c = crcT[(c ^ x) & 255] ^ (c >>> 8); return (c ^ -1) >>> 0 }
  const chunk = (type, data) => {
    const len = Buffer.alloc(4); len.writeUInt32BE(data.length)
    const td = Buffer.concat([Buffer.from(type), data])
    const c = Buffer.alloc(4); c.writeUInt32BE(crc(td))
    return Buffer.concat([len, td, c])
  }
  const ihdr = Buffer.alloc(13); ihdr.writeUInt32BE(w, 0); ihdr.writeUInt32BE(h, 4); ihdr[8] = 8; ihdr[9] = 2
  const row = Buffer.concat([Buffer.from([0]), Buffer.from(Array.from({ length: w }, () => [r, g, b]).flat())])
  const raw = Buffer.concat(Array.from({ length: h }, () => row))
  return Buffer.concat([Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]), chunk('IHDR', ihdr), chunk('IDAT', zlib.deflateSync(raw)), chunk('IEND', Buffer.alloc(0))])
}

async function signup(tgId, name, gender, types, interests, birth = '1999-05-10') {
  const login = await call('POST', '/api/auth/dev', { body: { telegram_id: tgId, name } })
  ok(login.status === 200 && login.json.token, `login ${name}`)
  const token = login.json.token
  let me = (await call('GET', '/api/me', { token })).json
  if (!me.atish_username) {
    const r = await call('PUT', '/api/me/username', { token, body: { suffix: name.toLowerCase() + '_x' } })
    ok(r.status === 200, `set username for ${name}`)
  }
  let r = await call('PATCH', '/api/me/profile', { token, body: { display_name: name, birth_date: birth, gender, bio: 'Hello from ' + name } })
  ok(r.status === 200, `create profile ${name}`)
  const loc = await call('GET', '/api/locations/cities?country=IT&q=genoa', { token })
  ok(loc.json.items.length >= 1, 'city search finds Genoa')
  r = await call('PATCH', '/api/me/profile', { token, body: { location_id: loc.json.items[0].id, connection_types: types, interests } })
  ok(r.status === 200, `set location/intent/interests ${name}`)
  const form = new FormData(); form.append('photo', new Blob([png(300, 400, [200, 80, 120])], { type: 'image/png' }), 'a.png')
  r = await call('POST', '/api/me/photos', { token, form })
  ok(r.status === 201 && r.json.url, `upload photo ${name}`)
  r = await call('POST', '/api/me/onboarding/complete', { token })
  ok(r.status === 200 && r.json.onboarding.complete, `complete onboarding ${name}`)
  return { token, id: r.json.id }
}

const run = Date.now() % 1000000
console.log('1. Onboarding')
const alice = await signup(900000 + run, 'Alice' + run, 'female', ['friends', 'relationship'], ['gaming', 'space', 'books'])
const bob = await signup(910000 + run, 'Bob' + run, 'male', ['friends'], ['gaming', 'music', 'books'])
const carol = await signup(920000 + run, 'Carol' + run, 'female', ['relationship'], ['gaming', 'music', 'cafes'])

console.log('2. Validation')
let r = await call('PATCH', '/api/me/profile', { token: alice.token, body: { birth_date: '2015-01-01' } })
ok(r.status === 400 && r.json.error.code === 'too_young', 'under-age rejected')
r = await call('PUT', '/api/me/username', { token: bob.token, body: { suffix: 'admin' } })
ok(r.status === 400, 'reserved username rejected')
r = await call('PUT', '/api/me/username', { token: bob.token, body: { suffix: 'ab' } })
ok(r.status === 400, 'short username rejected')
r = await call('GET', '/api/me', {})
ok(r.status === 401, 'unauthenticated rejected')

console.log('3. Username rules')
const me = (await call('GET', '/api/me', { token: alice.token })).json
ok(me.atish_username.startsWith('Atish_'), `username has prefix (${me.atish_username})`)
ok(me.telegram_user_id === undefined && me.birth_date !== undefined, 'owner view hides telegram id')

console.log('4. Discovery respects connection intent')
r = await call('GET', '/api/discover', { token: alice.token })
const aliceFeed = r.json.profiles.map(p => p.display_name)
ok(aliceFeed.some(n => n === 'Bob' + run), 'Alice (both) sees Bob (friends)')
ok(aliceFeed.some(n => n === 'Carol' + run), 'Alice (both) sees Carol (relationship)')
r = await call('GET', '/api/discover', { token: bob.token })
const bobFeed = r.json.profiles
ok(bobFeed.some(p => p.display_name === 'Alice' + run), 'Bob (friends) sees Alice')
ok(!bobFeed.some(p => p.display_name === 'Carol' + run), 'Bob (friends only) does NOT see Carol (relationship only)')
const aliceCard = bobFeed.find(p => p.display_name === 'Alice' + run)
ok(aliceCard.compat && aliceCard.compat.strength && !JSON.stringify(aliceCard).includes('telegram_user_id'), 'compat reasons present, no private fields')
ok(aliceCard.compat.reasons.some(x => x.type === 'interests'), 'shared-interest reason')
r = await call('POST', `/api/discover/${carol.id}/like`, { token: bob.token })
ok(r.status === 404 || r.status === 403, 'Bob cannot like incompatible Carol directly')

console.log('5. Like → match → chat')
const bobId = bobFeed.length ? (await call('GET', '/api/me', { token: bob.token })).json.id : null
r = await call('POST', `/api/discover/${aliceCard.id}/like`, { token: bob.token })
ok(r.status === 200 && r.json.matched === false, 'Bob likes Alice (no match yet)')
r = await call('POST', `/api/discover/${bobId}/like`, { token: alice.token })
ok(r.status === 200 && r.json.matched === true && r.json.match_id, 'Alice likes Bob → mutual match')
const matchId = r.json.match_id
r = await call('GET', '/api/matches', { token: bob.token })
ok(r.json.items.length === 1 && r.json.items[0].user.display_name === 'Alice' + run, 'match listed for Bob')
ok(JSON.stringify(r.json).includes('"friends"'), 'match connection type = friends (shared)')
r = await call('POST', `/api/chats/${matchId}/messages`, { token: alice.token, body: { body: 'Hi Bob!' } })
ok(r.status === 201, 'Alice sends message')
r = await call('GET', `/api/chats/${matchId}/messages`, { token: bob.token })
ok(r.json.items.length === 1 && r.json.items[0].body === 'Hi Bob!', 'Bob receives message')
r = await call('GET', '/api/chats', { token: bob.token })
ok(r.json.items[0].unread === 1, 'unread count = 1')
await call('POST', `/api/chats/${matchId}/read`, { token: bob.token })
r = await call('GET', '/api/chats/unread', { token: bob.token })
ok(r.json.unread === 0, 'unread cleared after read')
r = await call('GET', `/api/chats/${matchId}/messages`, { token: carol.token })
ok(r.status === 404, 'outsider cannot read chat')

console.log('6. Photos are signed URLs')
const photoUrl = r.json?.items ? null : (await call('GET', '/api/me', { token: alice.token })).json.photos[0].url
const img = await fetch(photoUrl)
ok(img.status === 200 && img.headers.get('content-type') === 'image/jpeg', 'signed photo URL serves JPEG')
const bad = await fetch(photoUrl.replace(/s=[0-9a-f]+/, 's=00000000000000000000000000000000'))
ok(bad.status === 403, 'tampered signature rejected')

console.log('7. Safety')
r = await call('POST', `/api/users/${carol.id}/report`, { token: alice.token, body: { reason: 'spam_scam', details: 'test report' } })
ok(r.status === 204, 'report filed')
r = await call('POST', `/api/users/${bobId}/block`, { token: alice.token })
ok(r.status === 204, 'Alice blocks Bob')
r = await call('GET', `/api/chats/${matchId}/messages`, { token: bob.token })
ok(r.status === 404, 'blocked chat is closed')
r = await call('GET', '/api/discover', { token: bob.token })
ok(!r.json.profiles.some(p => p.display_name === 'Alice' + run), 'blocked user disappears from discovery')

console.log('8. Premium & limits')
r = await call('GET', '/api/billing/plans', { token: alice.token })
ok(r.status === 200 && Array.isArray(r.json.items), 'plans endpoint works')
r = await call('GET', '/api/likes/received', { token: alice.token })
ok(r.status === 200 && r.json.locked === true, 'see-who-liked is locked for free users')

console.log('9. Admin')
r = await call('POST', '/api/admin/login', { body: { username: 'admin', password: 'wrong-wrong-wrong' } })
ok(r.status === 401, 'wrong admin password rejected')
r = await call('POST', '/api/admin/login', { body: { username: 'admin', password: ADMIN_PASSWORD } })
ok(r.status === 200, 'admin login')
const admin = r.json.token
r = await call('GET', '/api/admin/stats', { token: admin })
ok(r.status === 200 && r.json.users_total >= 3, 'stats')
r = await call('GET', '/api/admin/stats', { token: alice.token })
ok(r.status === 403, 'normal user cannot reach admin API')
r = await call('GET', '/api/admin/users?q=Carol', { token: admin })
ok(r.json.total >= 1, 'admin user search')
r = await call('GET', '/api/admin/reports', { token: admin })
const rep = r.json.items[0]
ok(rep && rep.reason === 'spam_scam', 'report visible to admin')
r = await call('PATCH', `/api/admin/reports/${rep.id}`, { token: admin, body: { status: 'resolved', resolution: 'spam', action: 'suspend' } })
ok(r.status === 204, 'resolve report with suspension')
r = await call('GET', '/api/me', { token: carol.token })
ok(r.status === 403, 'suspended user is locked out immediately')
r = await call('PATCH', `/api/admin/users/${carol.id}`, { token: admin, body: { status: 'active' } })
ok(r.status === 204, 'admin reactivates user')
r = await call('POST', '/api/admin/catalog/interests', { token: admin, body: { slug: 'smoke' + run, category: 'Test', name: 'Smoke ' + run, emoji: '🧪' } })
ok(r.status === 201, 'admin creates interest')
const newId = r.json.id
r = await call('PATCH', `/api/admin/catalog/interests/${newId}`, { token: admin, body: { name: 'Smoke renamed' } })
ok(r.status === 200 && r.json.name === 'Smoke renamed', 'admin edits interest')
r = await call('DELETE', `/api/admin/catalog/interests/${newId}`, { token: admin })
ok(r.status === 204, 'admin deletes interest')
r = await call('PUT', '/api/admin/settings/free_daily_likes', { token: admin, body: { value: 5 } })
ok(r.status === 204, 'admin changes setting')
r = await call('PUT', '/api/admin/settings/free_daily_likes', { token: admin, body: { value: 60 } })
r = await call('POST', `/api/admin/users/${carol.id}/premium`, { token: admin, body: { plan: 'premium_month', days: 30 } })
ok(r.status === 204, 'admin grants premium')
r = await call('GET', '/api/me', { token: carol.token })
ok(r.json.entitlements.premium === true, 'user is premium after grant')
r = await call('GET', '/api/admin/audit', { token: admin })
ok(r.json.items.length > 3, 'audit log records admin actions')

console.log('10. Account deletion')
r = await call('DELETE', '/api/me', { token: bob.token, body: { confirm: 'nope' } })
ok(r.status === 400, 'deletion needs confirmation')
r = await call('DELETE', '/api/me', { token: bob.token, body: { confirm: 'DELETE' } })
ok(r.status === 204, 'account deleted')
r = await call('GET', '/api/me', { token: bob.token })
ok(r.status === 401, 'deleted account token is dead')
r = await call('GET', '/api/discover', { token: carol.token })
ok(!r.json.profiles.some(p => p.display_name === 'Bob' + run), 'deleted user gone from discovery')

console.log(failed ? `\n${failed} CHECK(S) FAILED` : '\nALL CHECKS PASSED')
process.exit(failed ? 1 : 0)
