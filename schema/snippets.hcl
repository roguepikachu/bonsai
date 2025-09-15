# Bonsai snippet service schema definition
# This file defines the desired database schema state

table "snippets" {
  schema = schema.public

  column "id" {
    null = false
    type = text
  }

  column "content" {
    null = false
    type = text
  }

  column "tags" {
    null    = false
    type    = jsonb
    default = "'[]'::jsonb"
  }

  column "created_at" {
    null = false
    type = timestamptz
  }

  column "expires_at" {
    null = true
    type = timestamptz
  }

  primary_key {
    columns = [column.id]
  }

  index "idx_snippets_created_at" {
    columns = [column.created_at]
    type    = BTREE
    # Use DESC ordering for efficient newest-first queries
    on {
      column = column.created_at
      desc   = true
    }
  }

  index "idx_snippets_expires_at" {
    columns = [column.expires_at]
    type    = BTREE
    # For cleanup queries and expiration filtering
  }

  index "idx_snippets_tags_gin" {
    columns = [column.tags]
    type    = GIN
    # GIN index for efficient JSONB tag queries
  }
}

schema "public" {
  comment = "Bonsai snippet service public schema"
}