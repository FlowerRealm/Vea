from __future__ import annotations

from typing import Iterator

from sqlmodel import Session, SQLModel, create_engine


DATABASE_URL = "sqlite:///./data.db"

engine = create_engine(
    DATABASE_URL,
    connect_args={"check_same_thread": False},
    pool_pre_ping=True,
)


def init_db() -> None:
    SQLModel.metadata.create_all(bind=engine)


def get_session() -> Iterator[Session]:
    with Session(engine) as session:
        yield session
