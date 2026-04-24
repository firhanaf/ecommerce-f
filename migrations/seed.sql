-- ============================================================
-- Seed Data — Category & Product
-- Run: psql -U postgres -d ecommerce_db -f migrations/seed.sql
-- ============================================================

-- ─── Categories ──────────────────────────────────────────────────────────────

INSERT INTO categories (id, name, slug, parent_id, is_active, created_at) VALUES
  ('a1000000-0000-0000-0000-000000000001', 'Elektronik',         'elektronik',          NULL,                                   true, NOW()),
  ('a1000000-0000-0000-0000-000000000002', 'Fashion',            'fashion',             NULL,                                   true, NOW()),
  ('a1000000-0000-0000-0000-000000000003', 'Makanan & Minuman',  'makanan-minuman',     NULL,                                   true, NOW()),
  ('a1000000-0000-0000-0000-000000000004', 'Olahraga',           'olahraga',            NULL,                                   true, NOW()),

  -- Sub-kategori Elektronik
  ('a1000000-0000-0000-0000-000000000011', 'Smartphone',         'smartphone',          'a1000000-0000-0000-0000-000000000001', true, NOW()),
  ('a1000000-0000-0000-0000-000000000012', 'Laptop',             'laptop',              'a1000000-0000-0000-0000-000000000001', true, NOW()),
  ('a1000000-0000-0000-0000-000000000013', 'Aksesoris',          'aksesoris-elektronik','a1000000-0000-0000-0000-000000000001', true, NOW()),

  -- Sub-kategori Fashion
  ('a1000000-0000-0000-0000-000000000021', 'Pakaian Pria',       'pakaian-pria',        'a1000000-0000-0000-0000-000000000002', true, NOW()),
  ('a1000000-0000-0000-0000-000000000022', 'Pakaian Wanita',     'pakaian-wanita',      'a1000000-0000-0000-0000-000000000002', true, NOW()),
  ('a1000000-0000-0000-0000-000000000023', 'Sepatu',             'sepatu',              'a1000000-0000-0000-0000-000000000002', true, NOW()),

  -- Sub-kategori Olahraga
  ('a1000000-0000-0000-0000-000000000041', 'Fitness',            'fitness',             'a1000000-0000-0000-0000-000000000004', true, NOW()),
  ('a1000000-0000-0000-0000-000000000042', 'Outdoor',            'outdoor',             'a1000000-0000-0000-0000-000000000004', true, NOW())
ON CONFLICT DO NOTHING;

-- ─── Products ─────────────────────────────────────────────────────────────────

INSERT INTO products (id, name, slug, description, category_id, is_active, created_at, updated_at) VALUES
  (
    'b1000000-0000-0000-0000-000000000001',
    'iPhone 15 Pro',
    'iphone-15-pro',
    'Smartphone flagship Apple dengan chip A17 Pro, kamera 48MP, dan layar Super Retina XDR 6.1 inci. Hadir dengan Dynamic Island dan tombol Action.',
    'a1000000-0000-0000-0000-000000000011',
    true, NOW(), NOW()
  ),
  (
    'b1000000-0000-0000-0000-000000000002',
    'Samsung Galaxy S24',
    'samsung-galaxy-s24',
    'Smartphone Android premium dengan Snapdragon 8 Gen 3, layar Dynamic AMOLED 2X 6.2 inci, dan kamera 50MP dengan AI enhancement.',
    'a1000000-0000-0000-0000-000000000011',
    true, NOW(), NOW()
  ),
  (
    'b1000000-0000-0000-0000-000000000003',
    'MacBook Air M3',
    'macbook-air-m3',
    'Laptop tipis dan ringan dengan chip Apple M3, layar Liquid Retina 13.6 inci, baterai hingga 18 jam, dan desain fanless.',
    'a1000000-0000-0000-0000-000000000012',
    true, NOW(), NOW()
  ),
  (
    'b1000000-0000-0000-0000-000000000004',
    'Kaos Polos Premium Unisex',
    'kaos-polos-premium-unisex',
    'Kaos polos berbahan cotton combed 30s ringspun yang lembut dan adem. Tersedia dalam berbagai pilihan warna dan ukuran.',
    'a1000000-0000-0000-0000-000000000021',
    true, NOW(), NOW()
  ),
  (
    'b1000000-0000-0000-0000-000000000005',
    'Sepatu Running Nike Air Max',
    'sepatu-running-nike-air-max',
    'Sepatu lari dengan teknologi Air Max untuk bantalan optimal. Cocok untuk lari harian maupun kompetisi.',
    'a1000000-0000-0000-0000-000000000023',
    true, NOW(), NOW()
  ),
  (
    'b1000000-0000-0000-0000-000000000006',
    'Dumbbell Set Adjustable 2-32kg',
    'dumbbell-set-adjustable',
    'Set dumbbell adjustable yang bisa disesuaikan beratnya dari 2kg hingga 32kg. Pengganti 15 pasang dumbbell, hemat tempat dan praktis.',
    'a1000000-0000-0000-0000-000000000041',
    true, NOW(), NOW()
  )
ON CONFLICT DO NOTHING;

-- ─── Product Variants ─────────────────────────────────────────────────────────

INSERT INTO product_variants (id, product_id, name, sku, price, stock, is_active, created_at, updated_at) VALUES
  -- iPhone 15 Pro
  ('c1000000-0000-0000-0000-000000000001', 'b1000000-0000-0000-0000-000000000001', '128GB - Natural Titanium', 'IPH15P-128-NT', 18999000, 15, true, NOW(), NOW()),
  ('c1000000-0000-0000-0000-000000000002', 'b1000000-0000-0000-0000-000000000001', '256GB - Natural Titanium', 'IPH15P-256-NT', 20999000, 10, true, NOW(), NOW()),
  ('c1000000-0000-0000-0000-000000000003', 'b1000000-0000-0000-0000-000000000001', '256GB - Black Titanium',   'IPH15P-256-BT', 20999000, 8,  true, NOW(), NOW()),
  ('c1000000-0000-0000-0000-000000000004', 'b1000000-0000-0000-0000-000000000001', '512GB - White Titanium',   'IPH15P-512-WT', 24999000, 5,  true, NOW(), NOW()),

  -- Samsung Galaxy S24
  ('c1000000-0000-0000-0000-000000000011', 'b1000000-0000-0000-0000-000000000002', '128GB - Marble Gray',  'SGS24-128-MG', 13999000, 20, true, NOW(), NOW()),
  ('c1000000-0000-0000-0000-000000000012', 'b1000000-0000-0000-0000-000000000002', '256GB - Marble Gray',  'SGS24-256-MG', 15999000, 12, true, NOW(), NOW()),
  ('c1000000-0000-0000-0000-000000000013', 'b1000000-0000-0000-0000-000000000002', '256GB - Cobalt Violet','SGS24-256-CV', 15999000, 8,  true, NOW(), NOW()),

  -- MacBook Air M3
  ('c1000000-0000-0000-0000-000000000021', 'b1000000-0000-0000-0000-000000000003', '8GB RAM / 256GB SSD - Midnight',    'MBA-M3-8-256-MN',  16999000, 10, true, NOW(), NOW()),
  ('c1000000-0000-0000-0000-000000000022', 'b1000000-0000-0000-0000-000000000003', '8GB RAM / 512GB SSD - Starlight',   'MBA-M3-8-512-SL',  19999000, 7,  true, NOW(), NOW()),
  ('c1000000-0000-0000-0000-000000000023', 'b1000000-0000-0000-0000-000000000003', '16GB RAM / 512GB SSD - Space Gray', 'MBA-M3-16-512-SG', 23999000, 5,  true, NOW(), NOW()),

  -- Kaos Polos
  ('c1000000-0000-0000-0000-000000000031', 'b1000000-0000-0000-0000-000000000004', 'S - Putih',  'KAO-S-WHT',  89000, 50, true, NOW(), NOW()),
  ('c1000000-0000-0000-0000-000000000032', 'b1000000-0000-0000-0000-000000000004', 'M - Putih',  'KAO-M-WHT',  89000, 50, true, NOW(), NOW()),
  ('c1000000-0000-0000-0000-000000000033', 'b1000000-0000-0000-0000-000000000004', 'L - Putih',  'KAO-L-WHT',  89000, 50, true, NOW(), NOW()),
  ('c1000000-0000-0000-0000-000000000034', 'b1000000-0000-0000-0000-000000000004', 'XL - Putih', 'KAO-XL-WHT', 89000, 30, true, NOW(), NOW()),
  ('c1000000-0000-0000-0000-000000000035', 'b1000000-0000-0000-0000-000000000004', 'S - Hitam',  'KAO-S-BLK',  89000, 50, true, NOW(), NOW()),
  ('c1000000-0000-0000-0000-000000000036', 'b1000000-0000-0000-0000-000000000004', 'M - Hitam',  'KAO-M-BLK',  89000, 50, true, NOW(), NOW()),
  ('c1000000-0000-0000-0000-000000000037', 'b1000000-0000-0000-0000-000000000004', 'L - Hitam',  'KAO-L-BLK',  89000, 50, true, NOW(), NOW()),
  ('c1000000-0000-0000-0000-000000000038', 'b1000000-0000-0000-0000-000000000004', 'XL - Hitam', 'KAO-XL-BLK', 89000, 30, true, NOW(), NOW()),

  -- Sepatu Nike
  ('c1000000-0000-0000-0000-000000000041', 'b1000000-0000-0000-0000-000000000005', 'Size 40 - Hitam', 'NIKE-AM-40-BLK', 1599000, 10, true, NOW(), NOW()),
  ('c1000000-0000-0000-0000-000000000042', 'b1000000-0000-0000-0000-000000000005', 'Size 41 - Hitam', 'NIKE-AM-41-BLK', 1599000, 10, true, NOW(), NOW()),
  ('c1000000-0000-0000-0000-000000000043', 'b1000000-0000-0000-0000-000000000005', 'Size 42 - Hitam', 'NIKE-AM-42-BLK', 1599000, 8,  true, NOW(), NOW()),
  ('c1000000-0000-0000-0000-000000000044', 'b1000000-0000-0000-0000-000000000005', 'Size 43 - Hitam', 'NIKE-AM-43-BLK', 1599000, 8,  true, NOW(), NOW()),
  ('c1000000-0000-0000-0000-000000000045', 'b1000000-0000-0000-0000-000000000005', 'Size 42 - Putih', 'NIKE-AM-42-WHT', 1599000, 6,  true, NOW(), NOW()),
  ('c1000000-0000-0000-0000-000000000046', 'b1000000-0000-0000-0000-000000000005', 'Size 43 - Putih', 'NIKE-AM-43-WHT', 1599000, 6,  true, NOW(), NOW()),

  -- Dumbbell
  ('c1000000-0000-0000-0000-000000000051', 'b1000000-0000-0000-0000-000000000006', '2-32kg Set',       'DB-ADJ-32', 3299000, 15, true, NOW(), NOW()),
  ('c1000000-0000-0000-0000-000000000052', 'b1000000-0000-0000-0000-000000000006', '2-24kg Set (Lite)', 'DB-ADJ-24', 2499000, 20, true, NOW(), NOW())

ON CONFLICT DO NOTHING;
