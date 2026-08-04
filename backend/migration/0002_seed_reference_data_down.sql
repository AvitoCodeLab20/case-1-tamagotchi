DELETE FROM activity_types
WHERE code IN ('feed', 'play', 'rest', 'browse_listings', 'save_listing', 'publish_listing');

DELETE FROM levels WHERE level BETWEEN 1 AND 10;
