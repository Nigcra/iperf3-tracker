import asyncio
import sys
sys.path.insert(0, 'C:\\Users\\Nigcra\\Desktop\\iperf-Tracker\\backend')

from app.core.database import get_db, init_db
from app.models.models import User
from sqlalchemy import select

async def create_admin():
    await init_db()
    async for db in get_db():
        # Check if users exist
        result = await db.execute(select(User))
        existing_user = result.scalars().first()
        
        if existing_user:
            print(f"Users already exist. First user: {existing_user.username}")
        else:
            # Create admin with pre-hashed password for "admin"
            # This hash is for the password "admin"
            admin = User(
                username='admin',
                email='admin@local',
                hashed_password='$2a$12$LGvZlLr0MqP6q3e1JKWh6.8QKJyL3FyP1W4d3yOzlrX5YpN8CMNKG',
                is_admin=True,
                is_active=True
            )
            db.add(admin)
            await db.commit()
            print('✅ Admin user created successfully!')
            print('Username: admin')
            print('Password: admin')
            print('\n⚠️  IMPORTANT: Change this password after first login!')
        break

if __name__ == '__main__':
    asyncio.run(create_admin())
