-- SQLite test database initialization
-- Create sample tables for testing

CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE posts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    content TEXT,
    published BOOLEAN DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT
);

CREATE TABLE post_categories (
    post_id INTEGER,
    category_id INTEGER,
    PRIMARY KEY (post_id, category_id),
    FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE CASCADE
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
(1, 'Getting Started with Rust', 'Rust is a systems programming language...', 1),
(1, 'Database Design Patterns', 'When designing databases...', 1),
(2, 'My Trip to Japan', 'Last month I visited Japan...', 1),
(2, 'Cooking at Home', 'During the pandemic...', 0),
(3, 'Remote Work Tips', 'Working from home can be challenging...', 1);

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
WHERE p.published = 1;
