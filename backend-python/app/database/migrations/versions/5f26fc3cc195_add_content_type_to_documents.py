"""add content_type to documents

Revision ID: 5f26fc3cc195
Revises: 4e15eb2bb094
Create Date: 2025-12-30 06:35:00.000000

"""

from typing import Sequence, Union

import sqlalchemy as sa
from alembic import op

# revision identifiers, used by Alembic.
revision: str = "5f26fc3cc195"
down_revision: Union[str, Sequence[str], None] = "4e15eb2bb094"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    """Upgrade schema."""
    op.add_column(
        "documents",
        sa.Column(
            "content_type", sa.String(), nullable=True, server_default="application/octet-stream"
        ),
    )


def downgrade() -> None:
    """Downgrade schema."""
    op.drop_column("documents", "content_type")
