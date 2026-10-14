-- +goose Up
ALTER TABLE posts
  ALTER COLUMN content_md TYPE JSONB
  USING jsonb_build_object(
    'version', 1,
    'doc', jsonb_build_object(
      'type', 'doc',
      'content', jsonb_build_array(
        jsonb_build_object(
          'type', 'paragraph',
          'content', jsonb_build_array(
            jsonb_build_object('type', 'text', 'text', content_md)
          )
        )
      )
    )
  );
ALTER TABLE posts RENAME COLUMN content_md TO content;
ALTER TABLE posts ADD CONSTRAINT posts_content_is_object CHECK (jsonb_typeof(content) = 'object');

-- +goose Down
ALTER TABLE posts DROP CONSTRAINT posts_content_is_object;
ALTER TABLE posts RENAME COLUMN content TO content_md;
ALTER TABLE posts ALTER COLUMN content_md TYPE TEXT USING content_md::text;