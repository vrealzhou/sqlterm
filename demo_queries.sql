-- Demo queries for SQLTerm conversation mode testing

-- Query 1: Get all users
SELECT id, username, email FROM users LIMIT 5;

-- Query 2: Count posts by user
SELECT u.username, COUNT(p.id) as post_count 
FROM users u 
LEFT JOIN posts p ON u.id = p.user_id 
GROUP BY u.id, u.username 
ORDER BY post_count DESC 
LIMIT 10;

-- Query 3: Recent posts with category info
SELECT p.title, p.content, c.name as category, u.username as author
FROM posts p
JOIN categories c ON p.category_id = c.id
JOIN users u ON p.user_id = u.id
WHERE p.created_at > DATE_SUB(NOW(), INTERVAL 30 DAY)
ORDER BY p.created_at DESC
LIMIT 5;