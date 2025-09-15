-- Create "snippets" table
CREATE TABLE "public"."snippets" (
  "id" text NOT NULL,
  "content" text NOT NULL,
  "tags" jsonb NOT NULL DEFAULT '[]'::jsonb,
  "created_at" timestamptz NOT NULL,
  "expires_at" timestamptz NULL,
  PRIMARY KEY ("id")
);

-- Create index "idx_snippets_created_at" to table: "snippets"
CREATE INDEX CONCURRENTLY "idx_snippets_created_at" ON "public"."snippets" ("created_at" DESC);

-- Create index "idx_snippets_expires_at" to table: "snippets"
CREATE INDEX CONCURRENTLY "idx_snippets_expires_at" ON "public"."snippets" ("expires_at");

-- Create index "idx_snippets_tags_gin" to table: "snippets"
CREATE INDEX CONCURRENTLY "idx_snippets_tags_gin" ON "public"."snippets" USING GIN ("tags");