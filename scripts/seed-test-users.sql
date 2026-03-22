-- Comprehensive seed for AroundMe development/testing.
-- Creates 10 users in Craiova with skills, items, 50 posts (some with photos),
-- friend connections, direct + group DMs, trust endorsements, and notifications.
-- Password for all accounts: testpass123
--
-- Idempotent: safe to run multiple times. Cleans existing seed data first.
--
-- Run against the running container:
--   docker exec -i aroundme-postgres psql -U aroundme -d aroundme < scripts/seed-test-users.sql

DO $$
DECLARE
  test_user_ids UUID[];

  uid_alice   UUID;
  uid_bob     UUID;
  uid_charlie UUID;
  uid_diana   UUID;
  uid_eve     UUID;
  uid_frank   UUID;
  uid_grace   UUID;
  uid_henry   UUID;
  uid_iris    UUID;
  uid_jack    UUID;

  u        TEXT[];
  ud       TEXT[][];
  conv_id  UUID;
  v_post_id  UUID;
  cmt_id   UUID;
BEGIN

  -- ── PHASE 0: UPSERT USERS ───────────────────────────────────────────────
  ud := ARRAY[
    -- name, email, lat, lon, bio
    ARRAY['Alice Popescu',    'alice@test.com',   '44.3302', '23.7949',
          'Handyman enthusiast in City Centre. Happy to help with small repairs anytime.'],
    ARRAY['Bob Ionescu',      'bob@test.com',     '44.3180', '23.8052',
          'Amateur chef and passionate organic gardener. Calea Bucuresti area.'],
    ARRAY['Charlie Marin',    'charlie@test.com', '44.3358', '23.7701',
          'Full-stack developer and electronics tinkerer. Based near Calea Severinului.'],
    ARRAY['Diana Constantin', 'diana@test.com',   '44.3452', '23.8108',
          'Nurse by profession, community helper by calling. Brazda lui Novac.'],
    ARRAY['Eve Gheorghe',     'eve@test.com',     '44.3210', '23.7795',
          'Certified language teacher and bookworm. Organising neighbourhood events. Craiovita Noua.'],
    ARRAY['Frank Dumitrescu', 'frank@test.com',   '44.3095', '23.8215',
          'Licensed electrician (auth. ANRE). Free safety checks for seniors. Lapus.'],
    ARRAY['Grace Popa',       'grace@test.com',   '44.3412', '23.7860',
          'Baker and food enthusiast. Always have something in the oven. Rovine.'],
    ARRAY['Henry Stanescu',   'henry@test.com',   '44.3265', '23.8163',
          'Freelance photographer and graphic designer. Zona 1 Mai.'],
    ARRAY['Iris Luca',        'iris@test.com',    '44.3510', '23.7955',
          'Yoga instructor and wellness coach near Electroputere park.'],
    ARRAY['Jack Rusu',        'jack@test.com',    '44.3275', '23.7872',
          'Woodworker and welder. I build things. Parcul Romanescu area.']
  ];

  FOREACH u SLICE 1 IN ARRAY ud LOOP
    INSERT INTO users (email, name, latitude, longitude, bio)
    VALUES (u[2], u[1], u[3]::DOUBLE PRECISION, u[4]::DOUBLE PRECISION, u[5])
    ON CONFLICT (email) DO UPDATE SET
      name      = EXCLUDED.name,
      latitude  = EXCLUDED.latitude,
      longitude = EXCLUDED.longitude,
      bio       = EXCLUDED.bio;

    INSERT INTO auth_passwords (user_id, password_hash)
    SELECT id, crypt('testpass123', gen_salt('bf')) FROM users WHERE email = u[2]
    ON CONFLICT (user_id) DO NOTHING;
  END LOOP;

  -- Fetch UUIDs
  SELECT id INTO uid_alice   FROM users WHERE email = 'alice@test.com';
  SELECT id INTO uid_bob     FROM users WHERE email = 'bob@test.com';
  SELECT id INTO uid_charlie FROM users WHERE email = 'charlie@test.com';
  SELECT id INTO uid_diana   FROM users WHERE email = 'diana@test.com';
  SELECT id INTO uid_eve     FROM users WHERE email = 'eve@test.com';
  SELECT id INTO uid_frank   FROM users WHERE email = 'frank@test.com';
  SELECT id INTO uid_grace   FROM users WHERE email = 'grace@test.com';
  SELECT id INTO uid_henry   FROM users WHERE email = 'henry@test.com';
  SELECT id INTO uid_iris    FROM users WHERE email = 'iris@test.com';
  SELECT id INTO uid_jack    FROM users WHERE email = 'jack@test.com';

  test_user_ids := ARRAY[
    uid_alice, uid_bob, uid_charlie, uid_diana, uid_eve,
    uid_frank, uid_grace, uid_henry, uid_iris, uid_jack
  ];

  -- ── PHASE 1: CLEAN DEPENDENT DATA (idempotency) ──────────────────────────
  DELETE FROM notifications          WHERE user_id = ANY(test_user_ids);
  DELETE FROM trust_endorsements     WHERE from_user_id = ANY(test_user_ids)
                                        OR to_user_id   = ANY(test_user_ids);
  DELETE FROM trust_interactions     WHERE provider_user_id  = ANY(test_user_ids)
                                        OR recipient_user_id = ANY(test_user_ids);
  DELETE FROM post_comment_reactions WHERE user_id = ANY(test_user_ids);
  DELETE FROM post_reactions         WHERE user_id = ANY(test_user_ids);
  DELETE FROM post_comments          WHERE user_id = ANY(test_user_ids);
  DELETE FROM posts                  WHERE user_id = ANY(test_user_ids);
  DELETE FROM friend_requests        WHERE sender_id   = ANY(test_user_ids)
                                        OR receiver_id = ANY(test_user_ids);
  -- Delete conversations where every participant is a test user
  DELETE FROM conversations
  WHERE id IN (
    SELECT cp.conversation_id
    FROM   conversation_participants cp
    GROUP  BY cp.conversation_id
    HAVING bool_and(cp.user_id = ANY(test_user_ids))
  );
  DELETE FROM user_skills WHERE user_id = ANY(test_user_ids);
  DELETE FROM user_items  WHERE user_id = ANY(test_user_ids);

  -- ── PHASE 2: SKILLS ──────────────────────────────────────────────────────
  INSERT INTO user_skills (user_id, tag) VALUES
    (uid_alice,   'plumbing'),
    (uid_alice,   'carpentry'),
    (uid_alice,   'painting'),
    (uid_bob,     'cooking'),
    (uid_bob,     'gardening'),
    (uid_bob,     'composting'),
    (uid_charlie, 'programming'),
    (uid_charlie, 'electronics'),
    (uid_charlie, '3d_printing'),
    (uid_diana,   'first_aid'),
    (uid_diana,   'nursing'),
    (uid_diana,   'cpr'),
    (uid_eve,     'teaching'),
    (uid_eve,     'translation'),
    (uid_eve,     'tutoring'),
    (uid_frank,   'electrician'),
    (uid_frank,   'plumbing'),
    (uid_frank,   'hvac'),
    (uid_grace,   'baking'),
    (uid_grace,   'cooking'),
    (uid_grace,   'catering'),
    (uid_henry,   'photography'),
    (uid_henry,   'graphic_design'),
    (uid_henry,   'video_editing'),
    (uid_iris,    'yoga'),
    (uid_iris,    'fitness'),
    (uid_iris,    'meditation'),
    (uid_jack,    'carpentry'),
    (uid_jack,    'welding'),
    (uid_jack,    'metalwork')
  ON CONFLICT (user_id, tag) DO NOTHING;

  -- ── PHASE 3: ITEMS ───────────────────────────────────────────────────────
  INSERT INTO user_items (user_id, name, description, category, available, match_tags) VALUES
    (uid_alice,   'Extension Ladder',       '6m aluminium ladder, good condition',                    'tools',   true,  ARRAY['tools', 'repair']),
    (uid_alice,   'Power Drill',            'Bosch 18V cordless, full bit set',                       'tools',   true,  ARRAY['tools', 'repair']),
    (uid_alice,   'Tile Cutter',            'Manual rail cutter for tiles up to 60cm',                'tools',   false, ARRAY['tools', 'repair']),
    (uid_bob,     'Garden Hoe',             'Long-handle hoe, good for weeding',                      'garden',  true,  ARRAY['garden', 'cleanup']),
    (uid_bob,     'Wheelbarrow',            '80L steel wheelbarrow',                                  'garden',  true,  ARRAY['garden', 'cleanup']),
    (uid_charlie, 'Soldering Station',      'Hakko FX-888D, temperature controlled',                  'tools',   true,  ARRAY['tools', 'repair']),
    (uid_charlie, 'Oscilloscope',           'Rigol DS1054Z 4-channel',                                'tools',   false, ARRAY['tools', 'repair']),
    (uid_charlie, 'Raspberry Pi 4 Kit',     'Pi 4 4GB, case, PSU, 32GB SD with Raspbian',            'other',   true,  ARRAY['tech', 'education']),
    (uid_diana,   'First Aid Kit',          'Comprehensive 200-piece professional kit',               'home',    true,  ARRAY['shelter', 'first_aid']),
    (uid_diana,   'Blood Pressure Monitor', 'Omron M3, digital, clinically validated',                'home',    true,  ARRAY['shelter', 'health']),
    (uid_diana,   'CPR Training Dummy',     'Adult manikin, carry bag included',                      'other',   true,  ARRAY['first_aid', 'training']),
    (uid_eve,     'Language Books',         'French, German, Spanish B1/B2 coursebooks',              'books',   true,  ARRAY['books']),
    (uid_eve,     'Projector',              'BenQ 1080p, great for outdoor movie nights',             'home',    true,  ARRAY['shelter', 'entertainment']),
    (uid_frank,   'Cable Tester',           'Fluke network + electrical cable tester',                'tools',   true,  ARRAY['tools', 'repair']),
    (uid_frank,   'Angle Grinder',          'Makita 115mm, grinding and cutting discs included',      'tools',   true,  ARRAY['tools', 'repair']),
    (uid_grace,   'Stand Mixer',            'KitchenAid 5Qt, all attachments',                        'kitchen', true,  ARRAY['food']),
    (uid_grace,   'Bread Machine',          'Panasonic SD-ZX2522, 12 programs',                       'kitchen', true,  ARRAY['food']),
    (uid_henry,   'Camera + Tripod',        'Sony A7III body + Manfrotto 190 tripod',                'other',   true,  ARRAY['photography']),
    (uid_henry,   'Lighting Kit',           '3x softbox studio lights + stands',                     'other',   true,  ARRAY['photography']),
    (uid_iris,    'Yoga Mats (x3)',         'Non-slip 6mm TPE mats',                                  'sports',  true,  ARRAY['sports', 'yoga']),
    (uid_iris,    'Resistance Bands',       'Set of 5 bands, various resistance levels',              'sports',  true,  ARRAY['sports', 'fitness']),
    (uid_iris,    'Foam Roller',            'High-density 90cm roller',                               'sports',  true,  ARRAY['sports', 'fitness']),
    (uid_jack,    'Router Table',           'Woodworking router table with adjustable fence',         'tools',   true,  ARRAY['tools', 'repair']),
    (uid_jack,    'Bench Vice',             '150mm engineer''s vice, floor-mounted',                  'tools',   true,  ARRAY['tools', 'repair']),
    (uid_jack,    'MIG Welder',             'Lincoln 140C MIG welder, 220V',                         'tools',   false, ARRAY['tools', 'metalwork']);

  -- ── PHASE 4: POSTS (50) ──────────────────────────────────────────────────

  -- EMERGENCY (13)
  INSERT INTO posts (user_id, kind, status, title, excerpt, body,
                     location_name, latitude, longitude,
                     tags, image_url, share_location, created_at) VALUES

  (uid_alice, 'emergency', 'active',
   'Water pipe burst in basement!',
   'Pipe under the stairs burst — water flooding fast. Need plumber ASAP.',
   'The cold-water pipe feeding the washing machine connection has burst. Water is already 5cm deep. I have turned off the main stop valve but need someone to help with a proper repair tonight.',
   'Bulevardul Nicolae Titulescu, Craiova', 44.3305, 23.7951,
   ARRAY['plumbing','emergency','water'], NULL, true, NOW() - INTERVAL '28 days'),

  (uid_diana, 'emergency', 'active',
   'Elderly neighbour collapsed — needs help now',
   'Mrs Popa on floor 3 collapsed. Ambulance called. Trained medics please come.',
   'Our neighbour (80+) collapsed in the hallway of our block. Ambulance is en route. If any trained first-aider or nurse is nearby please come to Str. Dezrobirii 14, bl. B3, sc. 2.',
   'Str. Dezrobirii, Craiova', 44.3458, 23.8105,
   ARRAY['medical','emergency','firstaid'],
   'https://picsum.photos/seed/med1/800/600', true, NOW() - INTERVAL '26 days'),

  (uid_bob, 'emergency', 'active',
   'Lost dog — golden retriever, answers to Rex',
   'Rex escaped through an open gate this morning. Last seen near Parcul Romanescu.',
   'Our 3-year-old golden retriever Rex went missing around 8am. He is friendly and neutered. Wearing a red collar with a phone tag. Please call 0722-000-001 if you spot him.',
   'Parcul Romanescu, Craiova', 44.3278, 23.7875,
   ARRAY['lost_pet','dog','emergency'],
   'https://picsum.photos/seed/dog1/800/600', true, NOW() - INTERVAL '25 days'),

  (uid_frank, 'emergency', 'resolved',
   'Gas smell in stairwell — bloc A4',
   'Strong gas smell on floors 2–4. Everyone evacuated. Awaiting Distrigaz.',
   'All residents of bloc A4 on Calea Lapusului have evacuated. Distrigaz team arrived and identified a leaking joint on the riser. Fixed the same day. All clear.',
   'Calea Lapusului, Craiova', 44.3100, 23.8220,
   ARRAY['gas','emergency','resolved'],
   NULL, true, NOW() - INTERVAL '24 days'),

  (uid_charlie, 'emergency', 'active',
   'Power outage — Str. Electroputere dark since midnight',
   'Entire street without power. CEZ called but no ETA given.',
   'The outage started at 00:15. About 12 households affected. CEZ ticket #48812 filed. Anyone with a generator they can lend would be a huge help — we have a baby and it is cold.',
   'Str. Electroputere, Craiova', 44.3512, 23.7958,
   ARRAY['electricity','outage','emergency'],
   NULL, true, NOW() - INTERVAL '22 days'),

  (uid_eve, 'emergency', 'active',
   'Flooding in park underpass — pedestrian danger',
   'The underpass near Craiovita Noua is completely flooded after last night''s rain.',
   'At least 40cm of water in the underpass. Kids were almost caught this morning. Reported to city hall but no action yet. Please avoid and share widely.',
   'Craiovita Noua underpass, Craiova', 44.3215, 23.7798,
   ARRAY['flood','infrastructure','emergency'],
   'https://picsum.photos/seed/flood1/800/600', true, NOW() - INTERVAL '20 days'),

  (uid_henry, 'emergency', 'active',
   'Stray dog pack near school — children at risk',
   '5–6 stray dogs near Scoala 11. One snapped at a child this morning.',
   'Third incident this week. I have photos and video. Filed a complaint with DGASPC but no response so far. If anyone knows the right contact please comment.',
   'Scoala Nr. 11, Craiova', 44.3268, 23.8166,
   ARRAY['stray_dogs','safety','emergency'],
   'https://picsum.photos/seed/dogs2/800/600', true, NOW() - INTERVAL '18 days'),

  (uid_iris, 'emergency', 'active',
   'Roof tiles fell onto pavement — nobody hurt yet',
   'Loose roof tiles on Str. Olteniei. Very dangerous for pedestrians.',
   'At least 4 large tiles came off the building at nr. 23. I cordoned off the area with rope but someone official needs to act quickly. The landlord is uncontactable.',
   'Str. Olteniei, Craiova', 44.3515, 23.7960,
   ARRAY['structural','safety','emergency'],
   NULL, true, NOW() - INTERVAL '16 days'),

  (uid_jack, 'emergency', 'active',
   'Tree fell and blocked Str. Brestei',
   'Large oak came down in last night''s storm. Road completely blocked.',
   'The tree is across both lanes. No injuries reported. Called 112 and city hall. If anyone with a chainsaw can help clear a path for emergency vehicles that would be great.',
   'Str. Brestei, Craiova', 44.3280, 23.7880,
   ARRAY['fallen_tree','road','emergency'],
   'https://picsum.photos/seed/tree1/800/600', true, NOW() - INTERVAL '14 days'),

  (uid_alice, 'emergency', 'active',
   'Smoke near trash bins behind bloc P7',
   'Smoke coming from the bin area. Not sure if it is contained.',
   'It started about 10 minutes ago. Flames look small but the bin is melting. Called 112. Please stay away from the area while fire brigade arrives.',
   'Str. Caracal, Craiova', 44.3308, 23.7955,
   ARRAY['fire','safety','emergency'],
   NULL, true, NOW() - INTERVAL '12 days'),

  (uid_bob, 'emergency', 'active',
   'Burst water main flooding Calea Bucuresti',
   'Large crack in the road, water gushing out. Traffic diverted.',
   'The main appeared to burst around 7am. Water is flowing into two basements. ApaCaraiova reported but no crew on site yet as of 8am. Avoid the area if possible.',
   'Calea Bucuresti, Craiova', 44.3182, 23.8053,
   ARRAY['water','infrastructure','emergency'],
   'https://picsum.photos/seed/watermain/800/600', true, NOW() - INTERVAL '10 days'),

  (uid_diana, 'emergency', 'active',
   'Allergic reaction — does anyone have an EpiPen nearby?',
   'Child having a severe allergic reaction. Ambulance called. EpiPen needed NOW.',
   'My daughter ate something at a neighbour''s party. Anaphylaxis symptoms developing. Ambulance ETA 12 min. If anyone within 5 min walk has an EpiPen please come immediately to Str. Dezrobirii 14.',
   'Str. Dezrobirii, Craiova', 44.3455, 23.8106,
   ARRAY['medical','allergy','emergency'],
   NULL, true, NOW() - INTERVAL '8 days'),

  (uid_charlie, 'emergency', 'active',
   'Elevator stuck — elderly resident trapped in bloc C12',
   'Lift stopped between floors 4 and 5. An elderly woman inside. Lift company called.',
   'She called down through the door. She is OK and not panicking but has mild claustrophobia. Lift company ETA 45 min. Anyone with elevator maintenance experience please DM immediately.',
   'Str. Fraternității, Craiova', 44.3361, 23.7704,
   ARRAY['elevator','emergency','elderly'],
   NULL, true, NOW() - INTERVAL '6 days'),

  -- RESOURCE (20)
  (uid_alice, 'resource', 'active',
   'Free plumbing help on weekends',
   'Can help with leaky taps, pipe connections, toilet repairs. No charge for neighbours.',
   'Hobbyist plumber with 8 years of experience. Happy to help neighbours with small jobs. Have all tools. DM me with your issue and address.',
   'City Centre, Craiova', 44.3302, 23.7949,
   ARRAY['plumbing','help','free'],
   NULL, true, NOW() - INTERVAL '27 days'),

  (uid_bob, 'resource', 'active',
   'Free vegetable seedlings — tomatoes, peppers, basil',
   'About 50 seedlings ready for transplant. Free to good homes.',
   'Started indoors from heirloom seeds. Varieties: Roma tomatoes, Kapia peppers, Genovese basil, and courgettes. Come pick up Saturday 10–12.',
   'Calea Bucuresti, Craiova', 44.3182, 23.8055,
   ARRAY['garden','seedlings','free','plants'],
   'https://picsum.photos/seed/seedlings/800/600', true, NOW() - INTERVAL '25 days' + INTERVAL '4 hours'),

  (uid_charlie, 'resource', 'active',
   'Free WiFi troubleshooting and router setup',
   'Having network issues? I can diagnose and fix most home network problems for free.',
   'I do this professionally. Common issues: slow speeds, dead zones, smart home setup, firewall config. I bring my own testing equipment. DM to arrange a visit.',
   'Calea Severinului, Craiova', 44.3360, 23.7705,
   ARRAY['internet','wifi','tech','free'],
   NULL, true, NOW() - INTERVAL '23 days'),

  (uid_diana, 'resource', 'active',
   'Lending blood pressure monitor — free',
   'Happy to lend my Omron BPM for a week if you need to track your readings.',
   'Fully calibrated. Just DM me your address and return it clean. Great if your GP has asked you to monitor at home.',
   'Brazda lui Novac, Craiova', 44.3455, 23.8110,
   ARRAY['health','medical','lending'],
   NULL, true, NOW() - INTERVAL '21 days'),

  (uid_eve, 'resource', 'active',
   'Free French tutoring for beginners',
   'Offering 1h free French lessons (A1–A2). In person or video call.',
   'Certified language teacher (10 years). Great for kids aged 10–16. Groups of up to 3 welcome. DM to book.',
   'Craiovita Noua, Craiova', 44.3212, 23.7797,
   ARRAY['teaching','french','tutoring','free'],
   'https://picsum.photos/seed/tutor1/800/600', true, NOW() - INTERVAL '19 days'),

  (uid_frank, 'resource', 'active',
   'Lending angle grinder — up to 1 week',
   'Makita 115mm with grinding and cutting discs. Collect from Lapus.',
   'Need a signed borrowing note for liability reasons. Good condition. Bring your own PPE. Available from Saturday.',
   'Calea Lapusului, Craiova', 44.3098, 23.8218,
   ARRAY['tools','lending','grinder'],
   NULL, true, NOW() - INTERVAL '17 days'),

  (uid_grace, 'resource', 'active',
   'Homemade bread — free for elderly neighbours',
   'Baking every Sunday. Happy to drop off a loaf to anyone elderly or housebound.',
   'Sourdough and whole wheat. I bake 6 loaves each Sunday and reserve 2 for neighbours in need. Just let me know your preferences.',
   'Rovine, Craiova', 44.3415, 23.7863,
   ARRAY['food','baking','free','community'],
   'https://picsum.photos/seed/bread1/800/600', true, NOW() - INTERVAL '15 days'),

  (uid_henry, 'resource', 'active',
   'Free professional headshots for job seekers',
   'Offering free headshot sessions for anyone job hunting.',
   'Bring a smart outfit. Sessions are 20 min at my studio. I deliver 3 edited images within 48h. Priority to students and recently unemployed. Book via DM.',
   'Zona 1 Mai, Craiova', 44.3268, 23.8165,
   ARRAY['photography','jobs','free'],
   'https://picsum.photos/seed/photo1/800/600', true, NOW() - INTERVAL '13 days'),

  (uid_iris, 'resource', 'active',
   'Free outdoor yoga — every Saturday 8am',
   'Community yoga session in Parcul Romanescu. All levels welcome.',
   'Bring your own mat (I have 3 spares). 45-min flow focusing on flexibility and stress relief. No booking needed — just show up at the main fountain.',
   'Parcul Romanescu, Craiova', 44.3275, 23.7873,
   ARRAY['yoga','fitness','free','outdoor'],
   'https://picsum.photos/seed/yoga1/800/600', true, NOW() - INTERVAL '11 days'),

  (uid_jack, 'resource', 'active',
   'Building raised garden beds — free labour',
   'Offering to build raised beds from reclaimed oak. Materials cost only (~30 RON).',
   'I have spare 20x4cm oak planks from a workshop project. Can build 2x1m beds. You pay for screws and corner brackets. DM with your garden dimensions.',
   'Parcul Romanescu area, Craiova', 44.3277, 23.7874,
   ARRAY['gardening','carpentry','community'],
   NULL, true, NOW() - INTERVAL '9 days'),

  (uid_alice, 'resource', 'active',
   '6m ladder available to borrow this week',
   'My extension ladder is free to borrow for roof or gutter work.',
   'Please return same day. Must be able to transport it yourself (3.5m folded). Collect from City Centre. DM first.',
   'City Centre, Craiova', 44.3303, 23.7950,
   ARRAY['tools','ladder','lending'],
   NULL, true, NOW() - INTERVAL '7 days'),

  (uid_bob, 'resource', 'active',
   'Surplus courgettes and cucumber — free',
   'Garden is producing way too much. Free veg to anyone nearby.',
   'About 8 courgettes and 12 cucumbers ready today. More coming this weekend. Just knock — gate is always open. Str. Calea Bucuresti 45.',
   'Calea Bucuresti, Craiova', 44.3183, 23.8054,
   ARRAY['food','garden','free','vegetables'],
   'https://picsum.photos/seed/veg1/800/600', true, NOW() - INTERVAL '5 days'),

  (uid_charlie, 'resource', 'active',
   'Lending Raspberry Pi 4 kit for school projects',
   'Pi 4 kit available for a student who needs it for a tech project.',
   'Full kit: Pi 4 4GB, case, PSU, 32GB SD with Raspbian. 2-week loan. DM with the project details.',
   'Calea Severinului, Craiova', 44.3360, 23.7703,
   ARRAY['tech','education','lending','raspberry_pi'],
   NULL, true, NOW() - INTERVAL '4 days'),

  (uid_diana, 'resource', 'active',
   'Free CPR / first-aid demo for your building',
   'I can run a 1h basic first aid session for your block. Free.',
   'Topics: CPR, choking response, wound dressing, calling emergency services. I bring a manikin. Groups of 5–15. Evening or weekend slots available.',
   'Brazda lui Novac, Craiova', 44.3453, 23.8109,
   ARRAY['firstaid','cpr','training','free'],
   'https://picsum.photos/seed/firstaid1/800/600', true, NOW() - INTERVAL '3 days'),

  (uid_eve, 'resource', 'active',
   'English conversation club — meet Thursdays',
   'Starting an informal English conversation group. All levels welcome.',
   'We meet at Cafeneaua Centrala on Thursdays at 18:30. Practice English in a relaxed setting. Free. First session this Thursday.',
   'City Centre, Craiova', 44.3305, 23.7953,
   ARRAY['english','language','community','meetup'],
   NULL, true, NOW() - INTERVAL '2 days'),

  (uid_frank, 'resource', 'active',
   'Free electrical safety checks for elderly residents',
   'Offering free home electrical inspections for seniors and those with disabilities.',
   'Licensed electrician (auth. ANRE). I check fuse box, visible wiring, sockets, and appliances. Free. ~30 min. Priority to over-65s.',
   'Calea Lapusului, Craiova', 44.3097, 23.8217,
   ARRAY['electrician','safety','elderly','free'],
   NULL, true, NOW() - INTERVAL '36 hours'),

  (uid_grace, 'resource', 'active',
   'Sharing finished compost — take as much as you want',
   'My hot compost bins are ready. Rich, dark compost free to take.',
   'Bring your own bags or buckets. Available most evenings and weekends. Great for balcony pots, raised beds, or garden top-dressing.',
   'Rovine, Craiova', 44.3414, 23.7862,
   ARRAY['garden','compost','free'],
   'https://picsum.photos/seed/compost1/800/600', true, NOW() - INTERVAL '30 hours'),

  (uid_henry, 'resource', 'active',
   'Looking to photograph vintage cameras — prints in exchange',
   'If you own an interesting vintage camera I would love to photograph it and give you prints.',
   'Working on a local heritage photography project. Leica, Zorki, Zenit, Electro 35 — anything interesting welcome. You keep the camera, I give you A4 prints. DM.',
   'Zona 1 Mai, Craiova', 44.3267, 23.8164,
   ARRAY['photography','vintage','culture'],
   'https://picsum.photos/seed/vintagecam/800/600', true, NOW() - INTERVAL '20 hours'),

  (uid_iris, 'resource', 'active',
   'Free pilates mat class — Tuesday evenings',
   'Starting a free pilates class at the park shelter. Limited to 8 people.',
   'Every Tuesday at 18:30. Bring a mat (I have 3 spares). Focused on core, posture, and lower back health. Great if you sit at a desk all day.',
   'Parcul Romanescu, Craiova', 44.3274, 23.7872,
   ARRAY['pilates','fitness','free','outdoor'],
   NULL, true, NOW() - INTERVAL '10 hours'),

  (uid_alice, 'resource', 'active',
   'Fence repair for elderly or housebound — free',
   'Offering to fix a garden or balcony fence for anyone who cannot manage it themselves.',
   'I have wood, screws, hinges, and a power drill. Free community service. Priority to elderly and disabled residents. DM with photos of the damage.',
   'City Centre, Craiova', 44.3301, 23.7948,
   ARRAY['carpentry','repair','elderly','free'],
   NULL, true, NOW() - INTERVAL '5 hours'),

  -- EVENT (17)
  (uid_grace, 'event', 'active',
   'Neighbourhood bake sale — Sunday 2pm',
   'Community bake sale at Piata Centrala. Proceeds go to the local animal shelter.',
   'Bring homemade cakes, biscuits, jams, or breads. Set up from 13:30. Tables provided. We expect 15+ contributors. Shelter coordinator will be there to receive donations.',
   'Piata Centrala, Craiova', 44.3302, 23.7948,
   ARRAY['event','food','community','charity'],
   'https://picsum.photos/seed/bakesale/800/600', true, NOW() - INTERVAL '23 days'),

  (uid_henry, 'event', 'active',
   'Photo walk — documenting old Craiova architecture',
   'Group photography walk through the historic centre this Saturday 10am.',
   'We will visit Palatul de Justitie, Casa Baniasa, and the old Jewish quarter. Tips on architectural photography included. Phone cameras welcome too. ~2.5h walk.',
   'Piata Mihai Viteazu, Craiova', 44.3310, 23.7942,
   ARRAY['photography','culture','walk','event'],
   'https://picsum.photos/seed/photowalk/800/600', true, NOW() - INTERVAL '21 days'),

  (uid_iris, 'event', 'active',
   'Community clean-up — Parcul Romanescu',
   'Organising a park clean-up. Gloves and bags provided.',
   'Meet at the main entrance at 9am Saturday. We split into teams and cover the park in 2h. The council has agreed to send a truck for collected waste.',
   'Parcul Romanescu, Craiova', 44.3276, 23.7872,
   ARRAY['cleanup','park','volunteer','event'],
   'https://picsum.photos/seed/cleanup1/800/600', true, NOW() - INTERVAL '19 days'),

  (uid_jack, 'event', 'active',
   'DIY woodworking workshop — beginner friendly',
   'Free 3h woodworking intro this Sunday. Tools and wood provided.',
   'Learn to use hand tools safely, read a plan, and build a small shelf. Max 6 people. Ages 16+. Wear clothes you can get dirty.',
   'Parcul Romanescu area, Craiova', 44.3279, 23.7876,
   ARRAY['woodworking','workshop','diy','event'],
   NULL, true, NOW() - INTERVAL '17 days'),

  (uid_alice, 'event', 'active',
   'Bloc P7 association meeting — parking and noise',
   'Meeting to discuss parking enforcement and late-night noise issues.',
   'Saturday at 11am in the ground-floor common room. Agenda: (1) new parking sticker scheme, (2) noise curfew proposals, (3) bin area upgrade. All floor reps please attend.',
   'Bloc P7, Str. Caracal, Craiova', 44.3306, 23.7952,
   ARRAY['community','meeting','block','event'],
   NULL, true, NOW() - INTERVAL '15 days'),

  (uid_bob, 'event', 'active',
   'Seed swap — bring saved seeds, take new ones',
   'Informal seed swap at the community garden Saturday afternoon.',
   'Bring labelled packets of seeds you have saved. Take anything you fancy. We will have tomatoes, peppers, herbs, and flowers. Hot drinks provided.',
   'Gradina Comunitara Lapus, Craiova', 44.3100, 23.8222,
   ARRAY['gardening','seeds','community','event'],
   'https://picsum.photos/seed/seeds1/800/600', true, NOW() - INTERVAL '13 days'),

  (uid_charlie, 'event', 'active',
   'Repair café — bring broken electronics and small appliances',
   'Free repair session. Bring your broken toaster, lamp, or gadget.',
   'Skilled volunteers (electronics, sewing, mechanics) will try to fix items on the spot. At Biblioteca Judeteana reading room. Donations to the library welcome.',
   'Biblioteca Judeteana, Craiova', 44.3318, 23.7962,
   ARRAY['repair','electronics','community','sustainability','event'],
   'https://picsum.photos/seed/repaircafe/800/600', true, NOW() - INTERVAL '11 days'),

  (uid_diana, 'event', 'active',
   'Blood drive — Red Cross Dolj chapter',
   'Organising a community blood donation session. 20 slots available.',
   'Tuesday 15 April, 9am–1pm, at the Crucea Rosie Dolj office. Book via DM. Bring your ID. You will receive a light meal after donation. Every unit can save 3 lives.',
   'Crucea Rosie Dolj, Craiova', 44.3350, 23.7940,
   ARRAY['health','donation','blood','event'],
   'https://picsum.photos/seed/blood1/800/600', true, NOW() - INTERVAL '9 days'),

  (uid_eve, 'event', 'active',
   'Children''s story hour — multilingual',
   'Stories in Romanian, English, and French for children aged 4–8.',
   'Every Wednesday 17:00 at Biblioteca Judeteana children section. Free. We read the same story in three languages. Great for bilingual families.',
   'Biblioteca Judeteana, Craiova', 44.3318, 23.7962,
   ARRAY['children','language','reading','event'],
   NULL, true, NOW() - INTERVAL '7 days'),

  (uid_frank, 'event', 'active',
   'Block electrical audit day — sign up for a free check',
   'Full-day electrical audit for residents who want a free safety inspection.',
   'Saturday 19 April, 9am–5pm. I will visit each flat (30 min each). Priority: elderly residents, flats with older wiring. Max 12 slots. Sign up via DM.',
   'Calea Lapusului, Craiova', 44.3096, 23.8216,
   ARRAY['electrician','safety','event','community'],
   NULL, true, NOW() - INTERVAL '5 days'),

  (uid_grace, 'event', 'active',
   'Community BBQ — Parcul Romanescu picnic area',
   'Neighbourhood BBQ this Sunday from noon. Bring something to grill or a salad.',
   'Grills and charcoal provided. Bring your own meat/veg and a side dish to share. Kids welcome. Look for the AroundMe banner.',
   'Parcul Romanescu, Craiova', 44.3278, 23.7870,
   ARRAY['bbq','food','community','event'],
   'https://picsum.photos/seed/bbq1/800/600', true, NOW() - INTERVAL '3 days'),

  (uid_henry, 'event', 'active',
   'Free outdoor movie night — Piata Unirii',
   'Projecting a film on the Biblioteca wall. Saturday 9pm.',
   'Film decided by community vote (poll in comments). I supply the projector and sound system. Bring blankets and snacks. Starts at 21:00 when it gets dark.',
   'Piata Unirii, Craiova', 44.3320, 23.7965,
   ARRAY['movie','outdoor','community','event'],
   'https://picsum.photos/seed/movie1/800/600', true, NOW() - INTERVAL '2 days'),

  (uid_iris, 'event', 'active',
   'Mindfulness morning — free 1h session',
   'Guided meditation and breathing exercises in Parcul Romanescu.',
   'Sunday 7:30am under the old lime trees. 20 min breathwork, 30 min guided meditation, 10 min sharing circle. Bring a mat or blanket. All levels welcome.',
   'Parcul Romanescu, Craiova', 44.3274, 23.7871,
   ARRAY['meditation','wellness','event','outdoor'],
   'https://picsum.photos/seed/meditation1/800/600', true, NOW() - INTERVAL '1 day'),

  (uid_jack, 'event', 'active',
   'Street furniture repair day — Str. Brestei benches',
   'Community repair session for damaged benches and playground equipment.',
   'Partnered with the local council. They supply materials, we supply labour. Saturday 9am–1pm. Bring gloves and screwdrivers. Kids can help paint!',
   'Str. Brestei, Craiova', 44.3281, 23.7882,
   ARRAY['repair','community','volunteer','event'],
   'https://picsum.photos/seed/repair1/800/600', true, NOW() - INTERVAL '18 hours'),

  (uid_alice, 'event', 'resolved',
   'Water main repair — access notice for Str. Caracal',
   'Council notified residents of a 6h water shutoff on Thursday.',
   'Repair is complete. Water restored at 14:30, 2h ahead of schedule. Thanks to everyone who stockpiled water in advance.',
   'Str. Caracal, Craiova', 44.3307, 23.7953,
   ARRAY['water','infrastructure','notice','event'],
   NULL, true, NOW() - INTERVAL '30 days'),

  (uid_bob, 'event', 'active',
   'Composting workshop — turn kitchen waste into garden gold',
   'Free 2h composting workshop for beginners at the community garden.',
   'Learn: what to compost, what to avoid, hot vs cold composting, troubleshooting smells. Take a handful of finished compost home.',
   'Gradina Comunitara Lapus, Craiova', 44.3101, 23.8221,
   ARRAY['gardening','composting','workshop','event'],
   NULL, true, NOW() - INTERVAL '12 hours'),

  (uid_charlie, 'event', 'active',
   'Cyber safety talk for seniors — free workshop',
   'Helping older residents protect themselves from online scams.',
   'Thursday 14:30 at the senior center on Str. Unirii. Topics: phishing emails, secure passwords, WhatsApp scams, fake parcel SMS. No jargon — practical advice only.',
   'Centrul de Zi pentru Seniori, Craiova', 44.3330, 23.7955,
   ARRAY['tech','seniors','safety','workshop','event'],
   NULL, true, NOW() - INTERVAL '4 hours'),

  (uid_eve, 'event', 'active',
   'Neighbourhood book club — first meeting',
   'Starting a monthly book club. First pick: a Romanian classic.',
   'Meeting at Cafeneaua Centrala on Sunday 16:00. We are reading "Morometii" for the first session. Everyone welcome — even if you have not finished it. Come for the conversation.',
   'Cafeneaua Centrala, Craiova', 44.3308, 23.7947,
   ARRAY['books','culture','community','event'],
   'https://picsum.photos/seed/bookclub/800/600', true, NOW() - INTERVAL '2 hours');

  UPDATE posts
  SET category = 'uncategorized'
  WHERE user_id = ANY(test_user_ids);

  -- ── PHASE 5: POST REACTIONS ───────────────────────────────────────────────
  -- Various users react to the first 15 posts (ordered by created_at)
  INSERT INTO post_reactions (post_id, user_id)
  SELECT p.id, reactor
  FROM (
    SELECT id, ROW_NUMBER() OVER (ORDER BY created_at ASC) AS rn
    FROM posts WHERE user_id = ANY(test_user_ids)
  ) p
  CROSS JOIN UNNEST(ARRAY[
    uid_bob, uid_charlie, uid_diana, uid_eve,
    uid_frank, uid_grace, uid_henry, uid_iris, uid_jack
  ]) AS reactor
  WHERE p.rn <= 15
    AND reactor <> (SELECT user_id FROM posts WHERE id = p.id)
  ON CONFLICT DO NOTHING;

  -- Update reaction counts
  UPDATE posts
  SET reaction_count = (SELECT COUNT(*) FROM post_reactions WHERE post_reactions.post_id = posts.id)
  WHERE user_id = ANY(test_user_ids);

  -- ── PHASE 6: COMMENTS + REPLIES ──────────────────────────────────────────

  -- "Water pipe burst" comments
  SELECT id INTO v_post_id FROM posts
  WHERE user_id = uid_alice AND title = 'Water pipe burst in basement!' LIMIT 1;
  INSERT INTO post_comments (post_id, user_id, body) VALUES
    (v_post_id, uid_frank,   'I can come tonight after 19:00. DM me your address.'),
    (v_post_id, uid_bob,     'Hope you get it sorted! Make sure the stop valve is fully off.'),
    (v_post_id, uid_charlie, 'Check if your building has a shared stop-cock in the basement too.');
  SELECT id INTO cmt_id FROM post_comments
  WHERE post_comments.post_id = v_post_id AND user_id = uid_frank LIMIT 1;
  INSERT INTO post_comments (post_id, user_id, body, parent_id) VALUES
    (v_post_id, uid_alice, 'Frank, DM sent — thank you so much!', cmt_id);

  -- "Lost dog" comments
  SELECT id INTO v_post_id FROM posts
  WHERE user_id = uid_bob AND title LIKE 'Lost dog%' LIMIT 1;
  INSERT INTO post_comments (post_id, user_id, body) VALUES
    (v_post_id, uid_alice, 'Sharing in the block WhatsApp group right now. Hope Rex is found soon!'),
    (v_post_id, uid_grace, 'I walk through Parcul Romanescu every morning. Will keep an eye out.'),
    (v_post_id, uid_iris,  'I saw a golden retriever near the main fountain around 9am. Might be Rex!');
  SELECT id INTO cmt_id FROM post_comments
  WHERE post_comments.post_id = v_post_id AND user_id = uid_iris LIMIT 1;
  INSERT INTO post_comments (post_id, user_id, body, parent_id) VALUES
    (v_post_id, uid_bob, 'Iris! Running there now — what was he doing?', cmt_id);

  -- "Free outdoor yoga" comments
  SELECT id INTO v_post_id FROM posts
  WHERE user_id = uid_iris AND title LIKE 'Free outdoor yoga%' LIMIT 1;
  INSERT INTO post_comments (post_id, user_id, body) VALUES
    (v_post_id, uid_diana, 'I will be there this Saturday with a couple of colleagues from the hospital!'),
    (v_post_id, uid_grace, 'Can complete beginners with no experience join?'),
    (v_post_id, uid_alice, 'What happens if it rains?');
  SELECT id INTO cmt_id FROM post_comments
  WHERE post_comments.post_id = v_post_id AND user_id = uid_grace LIMIT 1;
  INSERT INTO post_comments (post_id, user_id, body, parent_id) VALUES
    (v_post_id, uid_iris, 'Absolutely — I structure the session for all levels!', cmt_id);
  SELECT id INTO cmt_id FROM post_comments
  WHERE post_comments.post_id = v_post_id AND user_id = uid_alice LIMIT 1;
  INSERT INTO post_comments (post_id, user_id, body, parent_id) VALUES
    (v_post_id, uid_iris, 'If it rains we move under the park pavilion. Same time, same spot.', cmt_id);

  -- "Community BBQ" comments
  SELECT id INTO v_post_id FROM posts
  WHERE user_id = uid_grace AND title LIKE 'Community BBQ%' LIMIT 1;
  INSERT INTO post_comments (post_id, user_id, body) VALUES
    (v_post_id, uid_grace,   'I will bring a big potato salad and homemade lemonade!'),
    (v_post_id, uid_jack,    'Bringing ribs and my portable grill in case the main ones are busy.'),
    (v_post_id, uid_bob,     'Fresh courgettes and peppers from the garden — marinated and ready to grill.'),
    (v_post_id, uid_charlie, 'Can someone bring a Bluetooth speaker?');
  SELECT id INTO cmt_id FROM post_comments
  WHERE post_comments.post_id = v_post_id AND user_id = uid_charlie LIMIT 1;
  INSERT INTO post_comments (post_id, user_id, body, parent_id) VALUES
    (v_post_id, uid_henry, 'I have a portable JBL Charge. Will bring it!', cmt_id);

  -- "Outdoor movie night" comments (vote poll)
  SELECT id INTO v_post_id FROM posts
  WHERE user_id = uid_henry AND title LIKE 'Free outdoor movie%' LIMIT 1;
  INSERT INTO post_comments (post_id, user_id, body) VALUES
    (v_post_id, uid_eve,    'Voting for "Filantropica" — great Romanian film and very fitting!'),
    (v_post_id, uid_alice,  'I vote for something light — how about "La Grande Vadrouille"?'),
    (v_post_id, uid_grace,  'Romanian or international? I vote for "Moromete Family".'),
    (v_post_id, uid_bob,    'Any Chaplin? Silent films work great outdoors.');

  -- ── PHASE 7: COMMENT REACTIONS ───────────────────────────────────────────
  -- Upvote Frank's plumbing comment
  SELECT id INTO cmt_id FROM post_comments
  WHERE user_id = uid_frank AND body LIKE 'I can come tonight%' LIMIT 1;
  INSERT INTO post_comment_reactions (comment_id, user_id)
  VALUES (cmt_id, uid_alice), (cmt_id, uid_bob), (cmt_id, uid_diana), (cmt_id, uid_grace)
  ON CONFLICT DO NOTHING;

  -- Upvote Iris's dog-sighting comment
  SELECT id INTO cmt_id FROM post_comments
  WHERE user_id = uid_iris AND body LIKE 'I saw a golden retriever%' LIMIT 1;
  INSERT INTO post_comment_reactions (comment_id, user_id)
  VALUES (cmt_id, uid_alice), (cmt_id, uid_charlie), (cmt_id, uid_diana)
  ON CONFLICT DO NOTHING;

  -- Upvote Iris's beginner reply
  SELECT id INTO cmt_id FROM post_comments
  WHERE user_id = uid_iris AND body LIKE 'Absolutely — I structure%' LIMIT 1;
  INSERT INTO post_comment_reactions (comment_id, user_id)
  VALUES (cmt_id, uid_grace), (cmt_id, uid_alice), (cmt_id, uid_eve)
  ON CONFLICT DO NOTHING;

  -- Update comment + reaction counts
  UPDATE post_comments
  SET reaction_count = (
    SELECT COUNT(*) FROM post_comment_reactions WHERE comment_id = post_comments.id
  );
  UPDATE posts
  SET comment_count = (SELECT COUNT(*) FROM post_comments WHERE post_comments.post_id = posts.id)
  WHERE user_id = ANY(test_user_ids);

  -- ── PHASE 8: FRIEND REQUESTS ─────────────────────────────────────────────
  INSERT INTO friend_requests (sender_id, receiver_id, status, message) VALUES
    (uid_alice,   uid_frank,   'accepted', 'Hey Frank, great to have a neighbour electrician!'),
    (uid_alice,   uid_bob,     'accepted', 'Hello Bob! Love your gardening posts.'),
    (uid_bob,     uid_grace,   'accepted', 'Fellow foodie! Let''s connect.'),
    (uid_charlie, uid_henry,   'accepted', 'Tech and creative — great combo!'),
    (uid_diana,   uid_iris,    'accepted', 'Both in wellness — let''s collaborate.'),
    (uid_diana,   uid_alice,   'accepted', 'Hi Alice, great community contributions!'),
    (uid_eve,     uid_charlie, 'accepted', 'Fellow educator meets tech person!'),
    (uid_frank,   uid_jack,    'accepted', 'Trades solidarity!'),
    (uid_grace,   uid_iris,    'accepted', 'Food and fitness — perfect combo!'),
    (uid_henry,   uid_bob,     'accepted', 'Love your green thumb posts, Bob.'),
    (uid_jack,    uid_alice,   'pending',  'Hi Alice, saw your carpentry posts — would love to connect!'),
    (uid_eve,     uid_diana,   'pending',  'Hi Diana, would love to connect!'),
    (uid_iris,    uid_charlie, 'pending',  'Tech and yoga — an unexpected but great combination!')
  ON CONFLICT (sender_id, receiver_id) DO NOTHING;

  -- ── PHASE 9: DIRECT CONVERSATIONS ────────────────────────────────────────

  -- Alice <-> Frank
  INSERT INTO conversations (kind, created_by, direct_pair)
  VALUES ('direct', uid_alice,
          LEAST(uid_alice::text, uid_frank::text) || ':' || GREATEST(uid_alice::text, uid_frank::text))
  ON CONFLICT DO NOTHING;
  SELECT id INTO conv_id FROM conversations
  WHERE direct_pair = LEAST(uid_alice::text, uid_frank::text) || ':' || GREATEST(uid_alice::text, uid_frank::text);
  INSERT INTO conversation_participants (conversation_id, user_id)
  VALUES (conv_id, uid_alice), (conv_id, uid_frank) ON CONFLICT DO NOTHING;
  INSERT INTO messages (conversation_id, sender_id, body) VALUES
    (conv_id, uid_alice, 'Hey Frank! Pipe issue sorted — you were a lifesaver last night.'),
    (conv_id, uid_frank, 'No problem at all! The joint was corroded, would have been worse if left.'),
    (conv_id, uid_alice, 'What do I owe you?'),
    (conv_id, uid_frank, 'Nothing — neighbours help neighbours. Buy me a coffee sometime.'),
    (conv_id, uid_alice, 'Deal! Would you be up for the bloc meeting this Saturday?'),
    (conv_id, uid_frank, 'Absolutely. What time?'),
    (conv_id, uid_alice, '11am in the ground-floor common room.');

  -- Bob <-> Grace
  INSERT INTO conversations (kind, created_by, direct_pair)
  VALUES ('direct', uid_bob,
          LEAST(uid_bob::text, uid_grace::text) || ':' || GREATEST(uid_bob::text, uid_grace::text))
  ON CONFLICT DO NOTHING;
  SELECT id INTO conv_id FROM conversations
  WHERE direct_pair = LEAST(uid_bob::text, uid_grace::text) || ':' || GREATEST(uid_bob::text, uid_grace::text);
  INSERT INTO conversation_participants (conversation_id, user_id)
  VALUES (conv_id, uid_bob), (conv_id, uid_grace) ON CONFLICT DO NOTHING;
  INSERT INTO messages (conversation_id, sender_id, body) VALUES
    (conv_id, uid_grace, 'Bob! Can I use some of your courgettes for the bake sale? Savoury courgette bread!'),
    (conv_id, uid_bob,   'Of course! Come by Saturday morning, I will have them ready.'),
    (conv_id, uid_grace, 'Amazing. I will bring you a loaf in exchange.'),
    (conv_id, uid_bob,   'Best deal I have made all week.');

  -- Charlie <-> Henry
  INSERT INTO conversations (kind, created_by, direct_pair)
  VALUES ('direct', uid_charlie,
          LEAST(uid_charlie::text, uid_henry::text) || ':' || GREATEST(uid_charlie::text, uid_henry::text))
  ON CONFLICT DO NOTHING;
  SELECT id INTO conv_id FROM conversations
  WHERE direct_pair = LEAST(uid_charlie::text, uid_henry::text) || ':' || GREATEST(uid_charlie::text, uid_henry::text);
  INSERT INTO conversation_participants (conversation_id, user_id)
  VALUES (conv_id, uid_charlie), (conv_id, uid_henry) ON CONFLICT DO NOTHING;
  INSERT INTO messages (conversation_id, sender_id, body) VALUES
    (conv_id, uid_henry,   'Charlie, can a Raspberry Pi run a timelapse camera setup?'),
    (conv_id, uid_charlie, 'Absolutely! Pi Camera v2 is perfect for that. I can set it up.'),
    (conv_id, uid_henry,   'Brilliant. I want to document the park restoration over several weeks.'),
    (conv_id, uid_charlie, 'Easy. I have a Pi 4 and a spare camera module ready to go.'),
    (conv_id, uid_henry,   'You are the best. Can we meet Thursday evening?'),
    (conv_id, uid_charlie, 'Sure, 19:00 at my place. I will have it pre-configured.'),
    (conv_id, uid_henry,   'Bringing wine as a thank you!');

  -- Diana <-> Iris
  INSERT INTO conversations (kind, created_by, direct_pair)
  VALUES ('direct', uid_diana,
          LEAST(uid_diana::text, uid_iris::text) || ':' || GREATEST(uid_diana::text, uid_iris::text))
  ON CONFLICT DO NOTHING;
  SELECT id INTO conv_id FROM conversations
  WHERE direct_pair = LEAST(uid_diana::text, uid_iris::text) || ':' || GREATEST(uid_diana::text, uid_iris::text);
  INSERT INTO conversation_participants (conversation_id, user_id)
  VALUES (conv_id, uid_diana), (conv_id, uid_iris) ON CONFLICT DO NOTHING;
  INSERT INTO messages (conversation_id, sender_id, body) VALUES
    (conv_id, uid_iris,  'Diana, would you do a short first aid talk after one of my yoga sessions?'),
    (conv_id, uid_diana, 'What a great idea! How many people usually come?'),
    (conv_id, uid_iris,  'Usually 8–15. Mostly women, 25–50 age range.'),
    (conv_id, uid_diana, 'Perfect. I can do 20 min on CPR and choking response. I will bring a manikin.'),
    (conv_id, uid_iris,  'Next Saturday after the 8am session? Finishes around 9:15.'),
    (conv_id, uid_diana, 'I will be there. See you at 9:15!');

  -- Frank <-> Jack
  INSERT INTO conversations (kind, created_by, direct_pair)
  VALUES ('direct', uid_frank,
          LEAST(uid_frank::text, uid_jack::text) || ':' || GREATEST(uid_frank::text, uid_jack::text))
  ON CONFLICT DO NOTHING;
  SELECT id INTO conv_id FROM conversations
  WHERE direct_pair = LEAST(uid_frank::text, uid_jack::text) || ':' || GREATEST(uid_frank::text, uid_jack::text);
  INSERT INTO conversation_participants (conversation_id, user_id)
  VALUES (conv_id, uid_frank), (conv_id, uid_jack) ON CONFLICT DO NOTHING;
  INSERT INTO messages (conversation_id, sender_id, body) VALUES
    (conv_id, uid_jack,  'Frank, can I borrow your cable tester? Installing lights in the workshop.'),
    (conv_id, uid_frank, 'Of course. Stop by any evening this week.'),
    (conv_id, uid_jack,  'Tomorrow 18:30?'),
    (conv_id, uid_frank, 'Perfect. Ring the left bell.'),
    (conv_id, uid_jack,  'Cheers. I will bring some scrap oak if you need any for a project.');

  -- Eve <-> Alice
  INSERT INTO conversations (kind, created_by, direct_pair)
  VALUES ('direct', uid_eve,
          LEAST(uid_eve::text, uid_alice::text) || ':' || GREATEST(uid_eve::text, uid_alice::text))
  ON CONFLICT DO NOTHING;
  SELECT id INTO conv_id FROM conversations
  WHERE direct_pair = LEAST(uid_eve::text, uid_alice::text) || ':' || GREATEST(uid_eve::text, uid_alice::text);
  INSERT INTO conversation_participants (conversation_id, user_id)
  VALUES (conv_id, uid_eve), (conv_id, uid_alice) ON CONFLICT DO NOTHING;
  INSERT INTO messages (conversation_id, sender_id, body) VALUES
    (conv_id, uid_alice, 'Hi Eve! Do you tutor adults in French? My partner wants to start from scratch.'),
    (conv_id, uid_eve,   'Of course! I have an adult beginner course. Weekly 1h sessions.'),
    (conv_id, uid_alice, 'Wonderful. When can he start?'),
    (conv_id, uid_eve,   'Next Tuesday 18:30 works great. Send me his name and I will prepare materials.');

  -- ── PHASE 10: GROUP CONVERSATIONS ────────────────────────────────────────

  -- Neighbourhood Watch
  INSERT INTO conversations (kind, name, created_by)
  VALUES ('group', 'Cartier Sigur - Watch', uid_diana)
  RETURNING id INTO conv_id;
  INSERT INTO conversation_participants (conversation_id, user_id)
  VALUES (conv_id, uid_diana), (conv_id, uid_alice), (conv_id, uid_frank),
         (conv_id, uid_jack),  (conv_id, uid_iris),  (conv_id, uid_henry)
  ON CONFLICT DO NOTHING;
  INSERT INTO messages (conversation_id, sender_id, body) VALUES
    (conv_id, uid_diana, 'Welcome to the neighbourhood watch group everyone!'),
    (conv_id, uid_alice, 'Great idea Diana. Should we set a patrol schedule?'),
    (conv_id, uid_frank, 'I can do Tuesday and Thursday evenings.'),
    (conv_id, uid_jack,  'Weekends for me. Saturday mornings especially.'),
    (conv_id, uid_henry, 'I will document anything suspicious with my camera.'),
    (conv_id, uid_iris,  'I walk the park every morning at 7am — happy to report anything.'),
    (conv_id, uid_diana, 'Perfect. Check in each morning: all quiet = GREEN, issue = AMBER.'),
    (conv_id, uid_alice, 'GREEN - Monday: all quiet on Str. Caracal.'),
    (conv_id, uid_frank, 'GREEN - Tuesday: nothing to report from Lapus.'),
    (conv_id, uid_iris,  'AMBER - Wednesday: van parked at park gate overnight. Plate noted: DJ-12-XYZ.');

  -- Community Events
  INSERT INTO conversations (kind, name, created_by)
  VALUES ('group', 'Events Craiova', uid_grace)
  RETURNING id INTO conv_id;
  INSERT INTO conversation_participants (conversation_id, user_id)
  VALUES (conv_id, uid_grace), (conv_id, uid_bob),  (conv_id, uid_eve),
         (conv_id, uid_henry), (conv_id, uid_iris), (conv_id, uid_charlie),
         (conv_id, uid_alice)
  ON CONFLICT DO NOTHING;
  INSERT INTO messages (conversation_id, sender_id, body) VALUES
    (conv_id, uid_grace,   'Hi all! Created this group to coordinate community events.'),
    (conv_id, uid_bob,     'Seed swap confirmed for Saturday 14:00 at the community garden.'),
    (conv_id, uid_eve,     'Story hour is Wednesday 17:00 at the library. Kids welcome!'),
    (conv_id, uid_henry,   'Photo walk Saturday 10:00 from Piata Mihai Viteazu.'),
    (conv_id, uid_iris,    'Saturday yoga is on as always — 8am park fountain. See you there!'),
    (conv_id, uid_grace,   'Bake sale Sunday 14:00 Piata Centrala. Need 2 more tables — anyone?'),
    (conv_id, uid_alice,   'I have a folding table you can use, Grace!'),
    (conv_id, uid_charlie, 'And I have another. I will bring it Sunday morning.'),
    (conv_id, uid_grace,   'You are all wonderful people.'),
    (conv_id, uid_bob,     'Also: composting workshop at the garden Sunday 10:00. All welcome!');

  -- Trades & Repairs
  INSERT INTO conversations (kind, name, created_by)
  VALUES ('group', 'Reparatii & Meserii', uid_frank)
  RETURNING id INTO conv_id;
  INSERT INTO conversation_participants (conversation_id, user_id)
  VALUES (conv_id, uid_frank), (conv_id, uid_alice), (conv_id, uid_jack),
         (conv_id, uid_charlie)
  ON CONFLICT DO NOTHING;
  INSERT INTO messages (conversation_id, sender_id, body) VALUES
    (conv_id, uid_frank,   'Tradespeople group — for coordinating tool loans and big repairs.'),
    (conv_id, uid_alice,   'Great idea. I have a ladder and drill available most weekends.'),
    (conv_id, uid_jack,    'Router table, vice, and welding gear available from my workshop.'),
    (conv_id, uid_charlie, 'Electronics, PCB repair, and network cabling — that is my territory.'),
    (conv_id, uid_frank,   'And I cover electrical. Between us we have most things covered!'),
    (conv_id, uid_alice,   'Anyone got a tile cutter? Need one for a bathroom job next weekend.'),
    (conv_id, uid_jack,    'I can get one from my mate''s workshop. Leave it with me.'),
    (conv_id, uid_frank,   'Also: free safety check day is Saturday. 12 slots open — please share.');

  -- Wellness & Health
  INSERT INTO conversations (kind, name, created_by)
  VALUES ('group', 'Sanatate & Wellness', uid_iris)
  RETURNING id INTO conv_id;
  INSERT INTO conversation_participants (conversation_id, user_id)
  VALUES (conv_id, uid_iris), (conv_id, uid_diana), (conv_id, uid_grace), (conv_id, uid_eve)
  ON CONFLICT DO NOTHING;
  INSERT INTO messages (conversation_id, sender_id, body) VALUES
    (conv_id, uid_iris,  'Created this for sharing wellness tips and coordinating health events.'),
    (conv_id, uid_diana, 'Great group! I will share the blood drive details here too.'),
    (conv_id, uid_grace, 'Food is wellness too — going to share some gut-healthy recipes here.'),
    (conv_id, uid_eve,   'And mental wellness — I read that bilingualism delays cognitive decline!'),
    (conv_id, uid_iris,  'Diana, could you do a breathing workshop after my next yoga session?'),
    (conv_id, uid_diana, 'Already planning it. I will bring pulse oximeters so people can see their stats.'),
    (conv_id, uid_grace, 'I will bake some healthy energy balls for everyone to try after!');

  -- ── PHASE 11: TRUST ENDORSEMENTS ─────────────────────────────────────────
  INSERT INTO trust_endorsements (from_user_id, to_user_id, note) VALUES
    (uid_alice,   uid_frank,   'Fixed my burst pipe at 20:00 on a Tuesday. Absolute hero.'),
    (uid_alice,   uid_bob,     'Always generous with garden produce. Real community spirit.'),
    (uid_bob,     uid_grace,   'Her bread is incredible and she genuinely cares about neighbours.'),
    (uid_charlie, uid_henry,   'Henry is professional, fast, and creative. Brilliant headshots.'),
    (uid_diana,   uid_iris,    'Iris brings enormous positivity to the neighbourhood.'),
    (uid_diana,   uid_alice,   'Alice is always the first to respond to any emergency post.'),
    (uid_eve,     uid_charlie, 'Charlie set up my home network for free. Incredibly generous.'),
    (uid_frank,   uid_jack,    'Jack''s work is solid. Lent his cable tester without hesitation.'),
    (uid_grace,   uid_iris,    'Iris''s yoga sessions are wonderful. So calming and uplifting.'),
    (uid_henry,   uid_bob,     'Bob gave me fresh veg three weeks in a row. A true legend.'),
    (uid_jack,    uid_frank,   'Frank is an excellent electrician and a genuinely good person.'),
    (uid_iris,    uid_diana,   'Diana is exactly the kind of neighbour everyone deserves.')
  ON CONFLICT (from_user_id, to_user_id) DO NOTHING;

  -- ── PHASE 12: TRUST INTERACTIONS ─────────────────────────────────────────
  INSERT INTO trust_interactions
    (provider_user_id, recipient_user_id, kind, status,
     note, feedback_note, positive_feedback, completed_at, feedback_at)
  VALUES
    (uid_frank, uid_alice, 'help', 'completed',
     'Emergency pipe repair',
     'Fixed perfectly, arrived within the hour', true,
     NOW() - INTERVAL '27 days', NOW() - INTERVAL '27 days' + INTERVAL '2 hours'),
    (uid_alice, uid_jack, 'lend', 'completed',
     'Lent ladder for roof gutters',
     'Returned clean and on time', true,
     NOW() - INTERVAL '6 days', NOW() - INTERVAL '5 days'),
    (uid_bob, uid_grace, 'lend', 'completed',
     'Lent courgettes for bake sale',
     'Brought back a loaf in return!', true,
     NOW() - INTERVAL '12 days', NOW() - INTERVAL '12 days' + INTERVAL '3 hours'),
    (uid_charlie, uid_henry, 'lend', 'completed',
     'Pi 4 kit for timelapse project',
     'Returned in perfect condition', true,
     NOW() - INTERVAL '3 days', NOW() - INTERVAL '2 days'),
    (uid_iris, uid_diana, 'help', 'completed',
     'First aid talk after yoga session',
     'Everyone loved it, very professional', true,
     NOW() - INTERVAL '1 day', NOW() - INTERVAL '20 hours'),
    (uid_frank, uid_jack, 'lend', 'pending',
     'Cable tester borrow', NULL, NULL, NULL, NULL)
  ON CONFLICT DO NOTHING;

  -- ── PHASE 13: TRUST SCORES ────────────────────────────────────────────────
  -- +10 per endorsement received, +5 per resolved post, +8 per completed positive interaction received
  UPDATE users SET trust_score = (
    (SELECT COUNT(*) * 10 FROM trust_endorsements WHERE to_user_id = users.id) +
    (SELECT COUNT(*) * 5  FROM posts WHERE user_id = users.id AND status = 'resolved') +
    (SELECT COUNT(*) * 8  FROM trust_interactions
     WHERE recipient_user_id = users.id AND status = 'completed' AND positive_feedback = true)
  )
  WHERE id = ANY(test_user_ids);

  -- ── PHASE 14: NOTIFICATIONS ───────────────────────────────────────────────
  INSERT INTO notifications (user_id, type, title, body, entity_id, is_read) VALUES
    -- Endorsements received
    (uid_alice,   'endorsement', 'New endorsement', 'Diana endorsed you as always first to respond.',        uid_diana::text,   true),
    (uid_frank,   'endorsement', 'New endorsement', 'Alice endorsed you for the emergency pipe repair.',     uid_alice::text,   false),
    (uid_bob,     'endorsement', 'New endorsement', 'Alice endorsed you for community generosity.',          uid_alice::text,   true),
    (uid_grace,   'endorsement', 'New endorsement', 'Bob endorsed your baking and community care.',          uid_bob::text,     false),
    (uid_henry,   'endorsement', 'New endorsement', 'Charlie endorsed your photography work.',               uid_charlie::text, true),
    (uid_iris,    'endorsement', 'New endorsement', 'Diana endorsed your positivity and community spirit.',  uid_diana::text,   false),
    (uid_charlie, 'endorsement', 'New endorsement', 'Eve endorsed your technical helpfulness.',              uid_eve::text,     true),
    (uid_jack,    'endorsement', 'New endorsement', 'Frank endorsed you for solid, reliable work.',          uid_frank::text,   false),
    (uid_diana,   'endorsement', 'New endorsement', 'Iris endorsed you for being a great neighbour.',        uid_iris::text,    true),
    -- Pending connection requests
    (uid_frank,   'connection_request', 'New connection request', 'Jack wants to connect with you.',         uid_jack::text,    false),
    (uid_diana,   'connection_request', 'New connection request', 'Eve wants to connect with you.',          uid_eve::text,     false),
    (uid_charlie, 'connection_request', 'New connection request', 'Iris wants to connect with you.',         uid_iris::text,    true),
    -- Post comments
    (uid_alice,   'post_comment', 'New comment on your post', 'Frank offered to come fix your pipe tonight.',    NULL, true),
    (uid_alice,   'post_comment', 'New comment on your post', 'Bob also commented on your pipe post.',           NULL, false),
    (uid_bob,     'post_comment', 'New comment on your post', 'Iris may have spotted Rex near the fountain!',    NULL, false),
    (uid_grace,   'post_comment', 'New comment on your post', 'Jack is bringing ribs to the BBQ!',               NULL, true),
    -- Skill matches from post-match agent
    (uid_alice,   'skill_match', 'Someone nearby needs your skills', 'A post near you matches your plumbing skills.',    NULL, false),
    (uid_frank,   'skill_match', 'Someone nearby needs your skills', 'A post near you matches your electrician skills.', NULL, false),
    (uid_diana,   'skill_match', 'Someone nearby needs your skills', 'A post near you matches your first_aid skills.',   NULL, true),
    (uid_alice,   'skill_match', 'Someone nearby needs your skills', 'A post near you matches your carpentry skills.',   NULL, false),
    (uid_iris,    'skill_match', 'Someone nearby needs your skills', 'A post near you matches your yoga skills.',        NULL, false)
  ON CONFLICT DO NOTHING;

  -- ── PHASE 15: SYNC last_message_at ───────────────────────────────────────
  UPDATE conversations c
  SET last_message_at = (
    SELECT MAX(m.created_at) FROM messages m WHERE m.conversation_id = c.id
  )
  WHERE id IN (
    SELECT DISTINCT conversation_id
    FROM conversation_participants
    WHERE user_id = ANY(test_user_ids)
  );

  RAISE NOTICE 'Seed complete: 10 users | 30 skills | 25 items | 50 posts | reactions + comments | 13 connections | 6 direct DMs | 4 group chats | 12 endorsements | 6 trust interactions | 21 notifications';
END $$;
