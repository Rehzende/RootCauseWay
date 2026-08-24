"""Shared pytest fixtures."""

import os

# Prevent settings from reading a real .env file during tests
os.environ.setdefault("REDIS_URL", "redis://localhost:6379/0")
os.environ.setdefault("BACKEND_API_URL", "http://localhost:8080/api/v1")
