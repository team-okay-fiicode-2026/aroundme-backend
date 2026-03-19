-- Seed 10 test users for development/testing.
-- All accounts use password: testpass123
--
-- Run against the running container:
--   docker exec -i aroundme-postgres psql -U aroundme -d aroundme < scripts/seed-test-users.sql

DO $$
DECLARE
  users_data TEXT[][] := ARRAY[
    -- name,  email,  lat,  long  (spread across Craiova neighbourhoods)
    ARRAY['Alice Popescu',    'alice@test.com',   '44.3302', '23.7949'],  -- City centre
    ARRAY['Bob Ionescu',      'bob@test.com',     '44.3180', '23.8052'],  -- Calea Bucuresti
    ARRAY['Charlie Marin',    'charlie@test.com', '44.3358', '23.7701'],  -- Calea Severinului
    ARRAY['Diana Constantin', 'diana@test.com',   '44.3452', '23.8108'],  -- Brazda lui Novac
    ARRAY['Eve Gheorghe',     'eve@test.com',     '44.3210', '23.7795'],  -- Craiovita Noua
    ARRAY['Frank Dumitrescu', 'frank@test.com',   '44.3095', '23.8215'],  -- Lapus
    ARRAY['Grace Popa',       'grace@test.com',   '44.3412', '23.7860'],  -- Rovine
    ARRAY['Henry Stanescu',   'henry@test.com',   '44.3265', '23.8163'],  -- 1 Mai
    ARRAY['Iris Luca',        'iris@test.com',    '44.3510', '23.7955'],  -- Electroputere
    ARRAY['Jack Rusu',        'jack@test.com',    '44.3275', '23.7872']   -- Parcul Romanescu
  ];
  u TEXT[];
  uid UUID;
BEGIN
  FOREACH u SLICE 1 IN ARRAY users_data LOOP
    INSERT INTO users (email, name, latitude, longitude)
    VALUES (u[2], u[1], u[3]::DOUBLE PRECISION, u[4]::DOUBLE PRECISION)
    ON CONFLICT (email) DO NOTHING;

    SELECT id INTO uid FROM users WHERE email = u[2];

    INSERT INTO auth_passwords (user_id, password_hash)
    VALUES (uid, crypt('testpass123', gen_salt('bf')))
    ON CONFLICT (user_id) DO NOTHING;
  END LOOP;

  RAISE NOTICE 'Seeded 10 test users in Craiova (password: testpass123)';
END $$;
