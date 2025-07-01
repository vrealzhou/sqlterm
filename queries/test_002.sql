select p.id, u.username, p.title, p.content from users u
JOIN posts p ON u.id=p.user_id;
