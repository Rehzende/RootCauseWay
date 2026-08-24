-- Vector search: pgvector embeddings for knowledge base + similar incidents
CREATE EXTENSION IF NOT EXISTS vector;

-- Embedding columns (nullable: rows without embeddings fall back to ILIKE search)
ALTER TABLE knowledge_base ADD COLUMN IF NOT EXISTS embedding vector(1536);
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS embedding vector(1536);

-- HNSW indexes for cosine-distance nearest-neighbor search
CREATE INDEX IF NOT EXISTS idx_knowledge_base_embedding ON knowledge_base USING hnsw (embedding vector_cosine_ops);
CREATE INDEX IF NOT EXISTS idx_incidents_embedding ON incidents USING hnsw (embedding vector_cosine_ops);
