-- +goose Up
-- +goose StatementBegin
DO $$
DECLARE
    admin_id UUID;
    role_id UUID;
BEGIN
    -- Check if admin exists
    IF NOT EXISTS (SELECT 1 FROM users WHERE email = 'admin@elysian.com') THEN
        INSERT INTO users (email, password_hash, name, is_active)
        VALUES ('admin@elysian.com', '$2a$10$xToTe0TecBkdb8AxHBmH6.7jndz.oXEJz2VSdT6Z3hbaEVI0zeLHO', 'System Admin', true)
        RETURNING id INTO admin_id;

        SELECT id INTO role_id FROM roles WHERE name = 'admin';

        IF role_id IS NOT NULL THEN
            INSERT INTO user_roles (user_id, role_id) VALUES (admin_id, role_id);
        END IF;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM users WHERE email = 'admin@elysian.com';
-- +goose StatementEnd
