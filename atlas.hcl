# Atlas configuration for Bonsai snippet service
# https://atlasgo.io/atlas-schema/hcl

# Database environment configurations
env "local" {
  src = "file://schema"
  url = getenv("DATABASE_URL")
  migration {
    dir = "file://atlas-migrations"
  }
  diff {
    concurrent_index {
      create = true
      drop = true
    }
  }
}

env "dev" {
  src = "file://schema"
  url = getenv("DATABASE_URL")
  migration {
    dir = "file://atlas-migrations"
  }
  diff {
    concurrent_index {
      create = true
      drop = true
    }
  }
}

env "staging" {
  src = "file://schema"
  url = getenv("DATABASE_URL")
  migration {
    dir = "file://atlas-migrations"
  }
  diff {
    concurrent_index {
      create = true
      drop = true
    }
  }
  # Enable lint checks for staging and production
  lint {
    destructive {
      error = true
    }
    data_depend {
      error = true
    }
    naming {
      error = true
    }
  }
}

env "production" {
  src = "file://schema"
  url = getenv("DATABASE_URL")
  migration {
    dir = "file://atlas-migrations"
  }
  diff {
    concurrent_index {
      create = true
      drop = true
    }
  }
  # Strict lint checks for production
  lint {
    destructive {
      error = true
    }
    data_depend {
      error = true
    }
    naming {
      error = true
    }
    # Require concurrent index creation in production
    concurrent_index {
      create = true
      drop = false  # Don't allow non-concurrent drops in production
    }
  }
}

