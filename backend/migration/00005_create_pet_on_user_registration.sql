-- +goose Up

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION create_pet_for_new_user()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO pets (user_id, name)
    VALUES (NEW.id, 'Авитоша');

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER users_create_pet
AFTER INSERT ON users
FOR EACH ROW
EXECUTE FUNCTION create_pet_for_new_user();


-- +goose Down

DROP TRIGGER IF EXISTS users_create_pet ON users;
DROP FUNCTION IF EXISTS create_pet_for_new_user();