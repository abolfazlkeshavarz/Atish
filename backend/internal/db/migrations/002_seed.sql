-- Default reference data. Everything here is editable from the admin panel.

INSERT INTO connection_types (slug, name, emoji, position) VALUES
 ('friends',      'Friends',      '🤝', 1),
 ('relationship', 'Relationship', '❤️', 2);

INSERT INTO friendship_kinds (slug, name, emoji, position) VALUES
 ('casual_friends',  'Casual friends',        '😊', 1),
 ('close_friends',   'Close friends',         '🫶', 2),
 ('study_buddy',     'Study buddy',           '📚', 3),
 ('gaming_buddy',    'Gaming buddy',          '🎮', 4),
 ('sports_partner',  'Sports partner',        '⚽', 5),
 ('gym_partner',     'Gym partner',           '🏋️', 6),
 ('travel_buddy',    'Travel buddy',          '✈️', 7),
 ('language_exchange','Language exchange',    '🗣️', 8),
 ('hobby_partner',   'Hobby partner',         '🎨', 9),
 ('going_out',       'Going out',             '🥂', 10),
 ('online_friends',  'Online friendship',     '💬', 11),
 ('networking',      'Professional networking','💼', 12),
 ('other',           'Other',                 '✨', 13);

INSERT INTO interests (slug, category, name, emoji, position) VALUES
 ('programming','Technology','Programming','💻',1), ('ai','Technology','AI','🤖',2), ('robotics','Technology','Robotics','🦾',3),
 ('cybersecurity','Technology','Cybersecurity','🛡️',4), ('hardware','Technology','Hardware','🔧',5), ('startups','Technology','Startups','🚀',6),
 ('movies','Entertainment','Movies','🎬',1), ('tv','Entertainment','TV shows','📺',2), ('anime','Entertainment','Anime','🍥',3),
 ('gaming','Entertainment','Gaming','🎮',4), ('music','Entertainment','Music','🎧',5), ('standup','Entertainment','Stand-up','🎤',6),
 ('fitness','Lifestyle','Fitness','💪',1), ('running','Lifestyle','Running','🏃',2), ('hiking','Lifestyle','Hiking','🥾',3),
 ('traveling','Lifestyle','Traveling','🧳',4), ('cooking','Lifestyle','Cooking','🍳',5), ('photography','Lifestyle','Photography','📷',6),
 ('yoga','Lifestyle','Yoga','🧘',7), ('cycling','Lifestyle','Cycling','🚴',8), ('football','Lifestyle','Football','⚽',9), ('swimming','Lifestyle','Swimming','🏊',10),
 ('books','Intellectual','Books','📖',1), ('science','Intellectual','Science','🔬',2), ('space','Intellectual','Space','🪐',3),
 ('psychology','Intellectual','Psychology','🧠',4), ('philosophy','Intellectual','Philosophy','🦉',5), ('history','Intellectual','History','🏛️',6),
 ('cafes','Social','Cafés','☕',1), ('concerts','Social','Concerts','🎸',2), ('events','Social','Events','🎟️',3),
 ('parties','Social','Parties','🪩',4), ('volunteering','Social','Volunteering','🤲',5), ('boardgames','Social','Board games','🎲',6),
 ('art','Creative','Art','🎨',1), ('writing','Creative','Writing','✍️',2), ('design','Creative','Design','🖌️',3),
 ('fashion','Creative','Fashion','👗',4), ('dancing','Creative','Dancing','💃',5), ('diy','Creative','DIY','🪚',6),
 ('pets','Nature & pets','Pets','🐶',1), ('nature','Nature & pets','Nature','🌿',2), ('camping','Nature & pets','Camping','⛺',3);

INSERT INTO languages (code, name, native_name) VALUES
 ('en','English','English'),('fa','Persian','فارسی'),('it','Italian','Italiano'),('es','Spanish','Español'),('fr','French','Français'),
 ('de','German','Deutsch'),('pt','Portuguese','Português'),('ru','Russian','Русский'),('tr','Turkish','Türkçe'),('ar','Arabic','العربية'),
 ('hi','Hindi','हिन्दी'),('bn','Bengali','বাংলা'),('ur','Urdu','اردو'),('zh','Chinese','中文'),('ja','Japanese','日本語'),
 ('ko','Korean','한국어'),('id','Indonesian','Bahasa Indonesia'),('vi','Vietnamese','Tiếng Việt'),('th','Thai','ไทย'),('nl','Dutch','Nederlands'),
 ('pl','Polish','Polski'),('uk','Ukrainian','Українська'),('ro','Romanian','Română'),('el','Greek','Ελληνικά'),('sv','Swedish','Svenska'),
 ('he','Hebrew','עברית'),('az','Azerbaijani','Azərbaycanca'),('kk','Kazakh','Қазақша'),('uz','Uzbek','Oʻzbekcha'),('ku','Kurdish','Kurdî'),
 ('cs','Czech','Čeština'),('hu','Hungarian','Magyar'),('sr','Serbian','Српски'),('fil','Filipino','Filipino'),('sw','Swahili','Kiswahili');

INSERT INTO personality_questions (key, text, options, position) VALUES
 ('social_style','At a social event, I usually…','[{"key":"many","label":"Talk to many people"},{"key":"few","label":"Talk to a few people"},{"key":"known","label":"Stay with people I know"},{"key":"depends","label":"Depends on the situation"}]',1),
 ('weekend','My ideal weekend is…','[{"key":"home","label":"Staying home"},{"key":"out","label":"Going out with friends"},{"key":"explore","label":"Exploring somewhere"},{"key":"sport","label":"Sports / activity"},{"key":"gaming","label":"Gaming"},{"key":"projects","label":"Personal projects"}]',2),
 ('communication','I prefer chatting…','[{"key":"occasionally","label":"Occasionally"},{"key":"daily","label":"Daily"},{"key":"often","label":"Several times a day"}]',3),
 ('planning','I usually prefer…','[{"key":"plan","label":"Planning everything"},{"key":"some","label":"Some planning"},{"key":"spontaneous","label":"Being spontaneous"}]',4),
 ('energy','My energy is more…','[{"key":"morning","label":"Early bird"},{"key":"night","label":"Night owl"},{"key":"flex","label":"Flexible"}]',5),
 ('humor','My humor is…','[{"key":"dry","label":"Dry & sarcastic"},{"key":"silly","label":"Silly & playful"},{"key":"dark","label":"Dark"},{"key":"gentle","label":"Gentle & wholesome"}]',6),
 ('conflict','When we disagree, I…','[{"key":"talk","label":"Talk it out right away"},{"key":"space","label":"Need some space first"},{"key":"avoid","label":"Avoid conflict"}]',7),
 ('adventure','New experiences make me…','[{"key":"excited","label":"Excited"},{"key":"curious","label":"Curious but careful"},{"key":"nervous","label":"A little nervous"}]',8),
 ('first_meet','For a first meetup I prefer…','[{"key":"coffee","label":"Coffee / a walk"},{"key":"activity","label":"An activity"},{"key":"group","label":"A group hangout"},{"key":"online","label":"Chatting online first"}]',9),
 ('values','What matters most to me…','[{"key":"loyalty","label":"Loyalty"},{"key":"fun","label":"Fun"},{"key":"growth","label":"Growth"},{"key":"calm","label":"Calm & stability"}]',10),
 ('music_vibe','My go-to vibe is…','[{"key":"chill","label":"Chill"},{"key":"party","label":"Party"},{"key":"focus","label":"Focus"},{"key":"mixed","label":"A bit of everything"}]',11),
 ('pets_pref','Pets are…','[{"key":"love","label":"My life"},{"key":"fine","label":"Nice to have"},{"key":"no","label":"Not for me"}]',12);

INSERT INTO plans (code, name, description, interval, price_cents, currency, stars_price, features, position) VALUES
 ('premium_month','Atish Plus','Everything in Atish, unlimited.','month', 499,'usd', 250, '{"unlimited_likes":true,"see_likes":true,"rewind":true,"badge":true}', 1),
 ('premium_year','Atish Plus — Yearly','Best value. Save 50%.','year', 2999,'usd', 1500, '{"unlimited_likes":true,"see_likes":true,"rewind":true,"badge":true}', 2);

INSERT INTO app_settings (key, value) VALUES
 ('min_age', '18'),
 ('max_photos', '3'),
 ('free_daily_likes', '60'),
 ('premium_enabled', 'true'),
 ('registration_open', 'true'),
 ('maintenance_mode', 'false'),
 ('pass_resurface_days', '30'),
 ('discovery_batch_size', '10'),
 ('banner', '""'),
 ('support_username', '""'),
 ('blocked_words', '["fuck","shit","bitch","nazi","rape","porn","sex","nigg","cunt","whore"]'),
 ('score_weights', '{"intent":40,"friendship":8,"relationship":8,"interests":20,"languages":8,"location":10,"personality":8,"availability":6,"practice":6,"liked_you":10}');

-- Starter locations (country > region > city > area). Users can add more from the app.
INSERT INTO locations (country_code, country, region, city, area) VALUES
 ('IT','Italy','Liguria','Genoa',''), ('IT','Italy','Liguria','Genoa','Albaro'), ('IT','Italy','Liguria','Genoa','Centro Storico'),
 ('IT','Italy','Liguria','Genoa','Nervi'), ('IT','Italy','Liguria','Genoa','Sampierdarena'),
 ('IT','Italy','Lombardy','Milan',''), ('IT','Italy','Lazio','Rome',''), ('IT','Italy','Piedmont','Turin',''),
 ('IT','Italy','Veneto','Venice',''), ('IT','Italy','Tuscany','Florence',''), ('IT','Italy','Emilia-Romagna','Bologna',''),
 ('IR','Iran','Tehran','Tehran',''), ('IR','Iran','Isfahan','Isfahan',''), ('IR','Iran','Razavi Khorasan','Mashhad',''),
 ('IR','Iran','Fars','Shiraz',''), ('IR','Iran','East Azerbaijan','Tabriz',''),
 ('US','United States','New York','New York City',''), ('US','United States','California','Los Angeles',''),
 ('US','United States','California','San Francisco',''), ('US','United States','Illinois','Chicago',''), ('US','United States','Texas','Austin',''),
 ('GB','United Kingdom','England','London',''), ('GB','United Kingdom','England','Manchester',''), ('GB','United Kingdom','Scotland','Edinburgh',''),
 ('DE','Germany','Berlin','Berlin',''), ('DE','Germany','Bavaria','Munich',''), ('DE','Germany','Hamburg','Hamburg',''),
 ('FR','France','Île-de-France','Paris',''), ('FR','France','Auvergne-Rhône-Alpes','Lyon',''),
 ('ES','Spain','Community of Madrid','Madrid',''), ('ES','Spain','Catalonia','Barcelona',''),
 ('TR','Turkey','Istanbul','Istanbul',''), ('TR','Turkey','Ankara','Ankara',''), ('TR','Turkey','Izmir','Izmir',''),
 ('AE','United Arab Emirates','Dubai','Dubai',''), ('CA','Canada','Ontario','Toronto',''), ('CA','Canada','Quebec','Montreal',''),
 ('NL','Netherlands','North Holland','Amsterdam',''), ('RU','Russia','Moscow','Moscow',''), ('IN','India','Maharashtra','Mumbai',''),
 ('IN','India','Delhi','New Delhi',''), ('BR','Brazil','São Paulo','São Paulo',''), ('AU','Australia','New South Wales','Sydney','');
