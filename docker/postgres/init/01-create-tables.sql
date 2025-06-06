-- Create sample tables for testing
\c testdb;

CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    email VARCHAR(100) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE posts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(200) NOT NULL,
    content TEXT,
    published BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE categories (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    description TEXT
);

CREATE TABLE post_categories (
    post_id INTEGER REFERENCES posts(id) ON DELETE CASCADE,
    category_id INTEGER REFERENCES categories(id) ON DELETE CASCADE,
    PRIMARY KEY (post_id, category_id)
);

-- Create indexes
CREATE INDEX idx_posts_user_id ON posts(user_id);
CREATE INDEX idx_posts_published ON posts(published);
CREATE INDEX idx_posts_created_at ON posts(created_at);

-- Insert sample data
INSERT INTO users (username, email) VALUES
('alice', 'alice@example.com'),
('bob', 'bob@example.com'),
('charlie', 'charlie@example.com');

INSERT INTO categories (name, description) VALUES
('Technology', 'Posts about technology and programming'),
('Lifestyle', 'Posts about lifestyle and personal experiences'),
('Travel', 'Posts about travel and adventures');

INSERT INTO posts (user_id, title, content, published) VALUES
(1, 'Getting Started with Rust', 'Rust is a systems programming language...', TRUE),
(1, 'Database Design Patterns', 'When designing databases...', TRUE),
(2, 'My Trip to Japan', 'Last month I visited Japan...', TRUE),
(2, 'Cooking at Home', 'During the pandemic...', FALSE),
(3, 'Remote Work Tips', 'Working from home can be challenging...', TRUE);

INSERT INTO post_categories (post_id, category_id) VALUES
(1, 1), (2, 1), (3, 3), (4, 2), (5, 2);

-- Create a view
CREATE VIEW published_posts AS
SELECT 
    p.id,
    p.title,
    p.content,
    u.username as author,
    p.created_at
FROM posts p
JOIN users u ON p.user_id = u.id
WHERE p.published = TRUE;
