-- +goose Up
INSERT INTO reward_definitions (
    code,
    title,
    description,
    reward_type,
    payload,
    trigger_type,
    trigger_value
)
VALUES
    ('level_5_delivery_10', 'Скидка 10% на доставку', 'Максимальная скидка — 200 рублей', 'discount',
        '{"type":"delivery_discount","percent":10,"max_amount_rub":200,"category_selection_required":false}', 'level', 5),
    ('level_10_promotion_20', 'Скидка 20% на продвижение', 'Для одного объявления, максимум 200 рублей', 'promotion',
        '{"type":"promotion_discount","percent":20,"max_amount_rub":200,"category_selection_required":false}', 'level', 10),
    ('level_15_category_15', 'Скидка 15% в категории', 'Категорию выбирает пользователь, максимум 300 рублей', 'discount',
        '{"type":"category_discount","percent":15,"max_amount_rub":300,"category_selection_required":true}', 'level', 15),
    ('level_20_promotion_300', '300 рублей на продвижение', 'Сертификат для одного объявления', 'promotion',
        '{"type":"promotion_certificate","amount_rub":300,"category_selection_required":false}', 'level', 20),
    ('level_25_price_highlight', 'Выделение цены', 'Выделение цены цветом на 7 дней', 'promotion',
        '{"type":"price_highlight","duration_days":7,"category_selection_required":false}', 'level', 25),
    ('level_30_promotion_500', '500 рублей на продвижение', 'Сертификат для одного объявления', 'promotion',
        '{"type":"promotion_certificate","amount_rub":500,"category_selection_required":false}', 'level', 30),
    ('level_35_xl_listing', 'XL-объявление', 'XL-объявление на 7 дней', 'promotion',
        '{"type":"xl_listing","duration_days":7,"category_selection_required":false}', 'level', 35),
    ('level_40_xl_badge', 'XL-объявление со значком', 'XL-объявление и значок на 7 дней', 'promotion',
        '{"type":"xl_listing_with_badge","duration_days":7,"category_selection_required":false}', 'level', 40),
    ('level_45_category_20', 'Скидка 20% в категории', 'Категорию выбирает пользователь, максимум 500 рублей', 'discount',
        '{"type":"category_discount","percent":20,"max_amount_rub":500,"category_selection_required":true}', 'level', 45),
    ('level_50_autoteka', 'Отчёт Автотеки', 'Один бесплатный отчёт', 'autoteka',
        '{"type":"autoteka_report","category_selection_required":false}', 'level', 50),
    ('streak_7_delivery_15', 'Скидка 15% на доставку', 'За серию в 7 дней, максимум 200 рублей', 'discount',
        '{"type":"delivery_discount","percent":15,"max_amount_rub":200,"category_selection_required":false}', 'streak', 7),
    ('streak_14_free_delivery', 'Бесплатная доставка', 'За серию в 14 дней, максимум 500 рублей', 'free_delivery',
        '{"type":"free_delivery","max_amount_rub":500,"category_selection_required":false}', 'streak', 14),
    ('leaderboard_5_promotion_1000', '1000 рублей на продвижение', 'Приз недельного топ-5%', 'promotion',
        '{"type":"promotion_certificate","amount_rub":1000,"category_selection_required":false}', 'leaderboard', 5),
    ('leaderboard_5_autoteka', 'Отчёт Автотеки', 'Приз недельного топ-5%', 'autoteka',
        '{"type":"autoteka_report","category_selection_required":false}', 'leaderboard', 5),
    ('leaderboard_5_free_delivery', 'Бесплатная доставка', 'Приз недельного топ-5%, максимум 1000 рублей', 'free_delivery',
        '{"type":"free_delivery","max_amount_rub":1000,"category_selection_required":false}', 'leaderboard', 5),
    ('leaderboard_10_promotion_500', '500 рублей на продвижение', 'Приз недельного топ-10%', 'promotion',
        '{"type":"promotion_certificate","amount_rub":500,"category_selection_required":false}', 'leaderboard', 10),
    ('leaderboard_10_xl_listing', 'XL-объявление', 'Приз недельного топ-10% на 7 дней', 'promotion',
        '{"type":"xl_listing","duration_days":7,"category_selection_required":false}', 'leaderboard', 10),
    ('leaderboard_10_free_delivery', 'Бесплатная доставка', 'Приз недельного топ-10%, максимум 500 рублей', 'free_delivery',
        '{"type":"free_delivery","max_amount_rub":500,"category_selection_required":false}', 'leaderboard', 10),
    ('leaderboard_15_promotion_200', '200 рублей на продвижение', 'Приз недельного топ-15%', 'promotion',
        '{"type":"promotion_certificate","amount_rub":200,"category_selection_required":false}', 'leaderboard', 15),
    ('leaderboard_15_delivery_20', 'Скидка 20% на доставку', 'Приз недельного топ-15%, максимум 200 рублей', 'discount',
        '{"type":"delivery_discount","percent":20,"max_amount_rub":200,"category_selection_required":false}', 'leaderboard', 15),
    ('leaderboard_15_category_10', 'Скидка 10% в категории', 'Приз недельного топ-15%, максимум 300 рублей', 'discount',
        '{"type":"category_discount","percent":10,"max_amount_rub":300,"category_selection_required":true}', 'leaderboard', 15)
ON CONFLICT (code) DO NOTHING;

INSERT INTO leaderboard_reward_options (tier, option_code, reward_id, sort_order)
SELECT option_data.tier, option_data.option_code, definition.id, option_data.sort_order
FROM (
    VALUES
        (5, 'leaderboard_5_promotion_1000', 1),
        (5, 'leaderboard_5_autoteka', 2),
        (5, 'leaderboard_5_free_delivery', 3),
        (10, 'leaderboard_10_promotion_500', 1),
        (10, 'leaderboard_10_xl_listing', 2),
        (10, 'leaderboard_10_free_delivery', 3),
        (15, 'leaderboard_15_promotion_200', 1),
        (15, 'leaderboard_15_delivery_20', 2),
        (15, 'leaderboard_15_category_10', 3)
) AS option_data(tier, option_code, sort_order)
JOIN reward_definitions AS definition ON definition.code = option_data.option_code
ON CONFLICT (tier, option_code) DO NOTHING;

INSERT INTO reward_categories (code, title)
VALUES
    ('transport', 'Транспорт'),
    ('real_estate', 'Недвижимость'),
    ('electronics', 'Электроника'),
    ('jobs', 'Работа'),
    ('services', 'Услуги')
ON CONFLICT (code) DO NOTHING;

-- +goose Down
DELETE FROM reward_categories
WHERE code IN ('transport', 'real_estate', 'electronics', 'jobs', 'services');

DELETE FROM leaderboard_reward_options
WHERE option_code IN (
    'leaderboard_5_promotion_1000',
    'leaderboard_5_autoteka',
    'leaderboard_5_free_delivery',
    'leaderboard_10_promotion_500',
    'leaderboard_10_xl_listing',
    'leaderboard_10_free_delivery',
    'leaderboard_15_promotion_200',
    'leaderboard_15_delivery_20',
    'leaderboard_15_category_10'
);

DELETE FROM reward_definitions
WHERE code IN (
    'level_5_delivery_10',
    'level_10_promotion_20',
    'level_15_category_15',
    'level_20_promotion_300',
    'level_25_price_highlight',
    'level_30_promotion_500',
    'level_35_xl_listing',
    'level_40_xl_badge',
    'level_45_category_20',
    'level_50_autoteka',
    'streak_7_delivery_15',
    'streak_14_free_delivery',
    'leaderboard_5_promotion_1000',
    'leaderboard_5_autoteka',
    'leaderboard_5_free_delivery',
    'leaderboard_10_promotion_500',
    'leaderboard_10_xl_listing',
    'leaderboard_10_free_delivery',
    'leaderboard_15_promotion_200',
    'leaderboard_15_delivery_20',
    'leaderboard_15_category_10'
);