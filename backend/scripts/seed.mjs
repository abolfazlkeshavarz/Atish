// Seeds demo users so Discover has people to show. DEV ONLY (needs DEV_AUTH=true on the API).
//   BASE=http://localhost:8080 node scripts/seed.mjs
import zlib from 'node:zlib'

const BASE = process.env.BASE || 'http://localhost:8080'

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
  if (!res.ok) throw new Error(`${method} ${path} → ${res.status} ${text}`)
  return json
}

const crcT = new Int32Array(256).map((_, n) => { let c = n; for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1; return c })
const crc = (buf) => { let c = -1; for (const x of buf) c = crcT[(c ^ x) & 255] ^ (c >>> 8); return (c ^ -1) >>> 0 }
const chunk = (type, data) => {
  const len = Buffer.alloc(4); len.writeUInt32BE(data.length)
  const td = Buffer.concat([Buffer.from(type), data]); const c = Buffer.alloc(4); c.writeUInt32BE(crc(td))
  return Buffer.concat([len, td, c])
}
// pretty gradient "portrait": two-colour diagonal gradient, soft circle head and shoulders
function portrait(w, h, [r1, g1, b1], [r2, g2, b2]) {
  const rows = []
  for (let y = 0; y < h; y++) {
    const row = Buffer.alloc(1 + w * 3)
    for (let x = 0; x < w; x++) {
      const t = (x / w + y / h) / 2
      let r = r1 + (r2 - r1) * t, g = g1 + (g2 - g1) * t, b = b1 + (b2 - b1) * t
      const hx = x - w / 2, hy = y - h * 0.36
      const head = hx * hx + hy * hy < (w * 0.17) ** 2
      const body = y > h * 0.55 && Math.abs(hx) < w * 0.32 * (1 + (y - h * 0.55) / h) && ((hx / (w * 0.42)) ** 2 + ((y - h * 1.05) / (h * 0.52)) ** 2) < 1
      if (head || body) { r = r * 0.35 + 255 * 0.65; g = g * 0.35 + 255 * 0.65; b = b * 0.35 + 255 * 0.65 }
      row[1 + x * 3] = r; row[2 + x * 3] = g; row[3 + x * 3] = b
    }
    rows.push(row)
  }
  const ihdr = Buffer.alloc(13); ihdr.writeUInt32BE(w, 0); ihdr.writeUInt32BE(h, 4); ihdr[8] = 8; ihdr[9] = 2
  return Buffer.concat([Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]), chunk('IHDR', ihdr), chunk('IDAT', zlib.deflateSync(Buffer.concat(rows))), chunk('IEND', Buffer.alloc(0))])
}

const PEOPLE = [
  { n: 'Giulia', a: 24, g: 'female', city: 'Genoa', area: 'Albaro', t: ['friends', 'relationship'], i: ['travel', 'photography', 'books', 'cafes', 'music'], l: [['it', 'native'], ['en', 'advanced']], bio: 'Architecture student. Coffee, long walks by the sea and bad puns.', c: [[255, 138, 61], [232, 50, 143]] },
  { n: 'Marco', a: 27, g: 'male', city: 'Genoa', area: 'Centro Storico', t: ['friends'], i: ['gaming', 'programming', 'anime', 'ai', 'boardgames'], l: [['it', 'native'], ['en', 'advanced']], bio: 'Backend dev. Looking for a co-op partner and someone to explore the carruggi with.', c: [[45, 212, 191], [14, 165, 233]], kinds: ['gaming_buddy', 'hobby_partner'] },
  { n: 'Sara', a: 23, g: 'female', city: 'Genoa', area: 'Albaro', t: ['friends'], i: ['gaming', 'space', 'books', 'science', 'anime'], l: [['en', 'native'], ['it', 'intermediate']], bio: 'Erasmus in Genoa! Teach me Italian and I’ll teach you terrible English jokes.', c: [[167, 139, 250], [236, 72, 153]], kinds: ['language_exchange', 'casual_friends'], practice: 'it' },
  { n: 'Luca', a: 29, g: 'male', city: 'Genoa', area: 'Nervi', t: ['relationship'], i: ['fitness', 'hiking', 'cooking', 'traveling', 'movies'], l: [['it', 'native']], bio: 'Chef by day, hiker on weekends. Looking for something real.', c: [[251, 146, 60], [239, 68, 68]], intention: 'long_term' },
  { n: 'Elena', a: 26, g: 'female', city: 'Genoa', area: 'Sampierdarena', t: ['friends', 'relationship'], i: ['art', 'design', 'photography', 'concerts', 'fashion'], l: [['it', 'native'], ['fr', 'advanced'], ['en', 'advanced']], bio: 'Graphic designer. Always up for an exhibition or a spontaneous aperitivo.', c: [[244, 114, 182], [129, 140, 248]], intention: 'getting_to_know' },
  { n: 'Davide', a: 31, g: 'male', city: 'Genoa', area: '', t: ['friends', 'relationship'], i: ['startups', 'ai', 'running', 'books', 'psychology'], l: [['it', 'native'], ['en', 'advanced']], bio: 'Founder & marathon-in-progress. Curious about people more than anything.', c: [[56, 189, 248], [99, 102, 241]], intention: 'not_sure' },
  { n: 'Chiara', a: 22, g: 'female', city: 'Milan', area: '', t: ['friends'], i: ['music', 'concerts', 'parties', 'fashion', 'cafes'], l: [['it', 'native'], ['en', 'intermediate']], bio: 'Milan girl who loves live music.', c: [[251, 113, 133], [251, 191, 36]], kinds: ['going_out'] },
  { n: 'Reza', a: 28, g: 'male', city: 'Genoa', area: 'Albaro', t: ['friends', 'relationship'], i: ['programming', 'cybersecurity', 'gaming', 'space', 'music'], l: [['fa', 'native'], ['en', 'advanced'], ['it', 'basic']], practice: 'it', bio: 'PhD student from Tehran. Persian tea ↔ Italian lessons?', c: [[20, 184, 166], [59, 130, 246]], intention: 'getting_to_know', kinds: ['language_exchange', 'study_buddy'] },
  { n: 'Anna', a: 25, g: 'female', city: 'Genoa', area: 'Centro Storico', t: ['relationship'], i: ['yoga', 'nature', 'pets', 'cooking', 'books'], l: [['it', 'native'], ['es', 'advanced']], bio: 'Vet student, cat person, slow-morning enthusiast.', c: [[134, 239, 172], [34, 197, 94]], intention: 'long_term' },
  { n: 'Tommaso', a: 30, g: 'male', city: 'Genoa', area: 'Albaro', t: ['friends'], i: ['football', 'fitness', 'swimming', 'movies', 'traveling'], l: [['it', 'native'], ['en', 'intermediate']], bio: 'Five-a-side on Thursdays — we are one player short!', c: [[250, 204, 21], [234, 88, 12]], kinds: ['sports_partner', 'gym_partner'] },
]

const STARS = (await call('GET', '/api/catalog')).catalog
console.log(`Seeding ${PEOPLE.length} demo users into ${BASE} …`)
let idx = 0
for (const p of PEOPLE) {
  idx++
  const login = await call('POST', '/api/auth/dev', { body: { telegram_id: 700000 + idx, name: p.n } })
  const token = login.token
  const me = await call('GET', '/api/me', { token })
  if (me.onboarding.complete) { console.log('  · exists:', p.n); continue }
  if (!me.atish_username) await call('PUT', '/api/me/username', { token, body: { suffix: p.n.toLowerCase() + '_demo' } })
  const year = new Date().getFullYear() - p.a - 1
  await call('PATCH', '/api/me/profile', { token, body: { display_name: p.n, birth_date: `${year}-03-15`, gender: p.g, bio: p.bio } })
  const cities = await call('GET', `/api/locations/cities?country=IT&q=${encodeURIComponent(p.city)}`, { token })
  let loc = cities.items[0]
  if (p.area) { const areas = await call('GET', `/api/locations/areas?country=IT&city=${encodeURIComponent(p.city)}`, { token }); loc = areas.items.find((a) => a.area === p.area) ?? loc }
  await call('PATCH', '/api/me/profile', { token, body: {
    location_id: loc.id, connection_types: p.t, interests: p.i.filter((s) => STARS.interests.some((x) => x.slug === s)),
    languages: p.l.map(([code, level]) => ({ code, level, wants_practice: p.practice === code })),
  } })
  await call('PATCH', '/api/me/preferences', { token, body: {
    friendship_kinds: p.t.includes('friends') ? (p.kinds ?? []) : [],
    preferences: { relationship_intention: p.t.includes('relationship') ? (p.intention ?? 'not_sure') : '', preferred_genders: ['everyone'], age_min: 18, age_max: 60, distance_scope: 'same_country' },
  } })
  const form = new FormData(); form.append('photo', new Blob([portrait(360, 480, p.c[0], p.c[1])], { type: 'image/png' }), 'p.png')
  await call('POST', '/api/me/photos', { token, form })
  await call('POST', '/api/me/onboarding/complete', { token })
  console.log('  ✓', p.n)
}
console.log('Done. Sign in with any telegram_id 700001…700010 via the dev login screen.')
