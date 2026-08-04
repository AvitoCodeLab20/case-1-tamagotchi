-- +goose Up
INSERT INTO levels (level, required_total_experience, title)
VALUES
    (1, 0, 'Новичок'),
    (2, 100, 'Знакомство'),
    (3, 250, 'Заботливый хозяин'),
    (4, 450, 'Искатель'),
    (5, 700, 'Постоянный пользователь'),
    (6, 1000, 'Опытный хозяин'),
    (7, 1400, 'Знаток Авито'),
    (8, 1900, 'Мастер находок'),
    (9, 2500, 'Эксперт'),
    (10, 3200, 'Легенда Авито')
ON CONFLICT (level) DO NOTHING;

INSERT INTO activity_types (
    code,
    title,
    description,
    category,
    base_experience,
    daily_limit,
    cooldown_seconds
)
VALUES
    ('feed', 'Покормить', 'Восстанавливает сытость питомца', 'care', 10, 3, 14400),
    ('play', 'Поиграть', 'Повышает настроение питомца', 'care', 10, 3, 10800),
    ('rest', 'Отдохнуть', 'Восстанавливает энергию питомца', 'care', 8, 2, 21600),
    ('browse_listings', 'Посмотреть объявления', 'Засчитывается после осмысленного просмотра подборки', 'avito_product', 15, 3, 3600),
    ('save_listing', 'Добавить в избранное', 'Связывает прогресс питомца с интересом к объявлению', 'avito_product', 20, 2, 0),
    ('publish_listing', 'Разместить объявление', 'Награждает за ключевое действие доски объявлений', 'avito_product', 60, 1, 0)
ON CONFLICT (code) DO NOTHING;

-- +goose Down
DELETE FROM activity_types
WHERE code IN ('feed', 'play', 'rest', 'browse_listings', 'save_listing', 'publish_listing');

DELETE FROM levels WHERE level BETWEEN 1 AND 10;
