import asyncio
import sys
sys.path.insert(0, 'C:\\Users\\Nigcra\\Desktop\\iperf-Tracker\\backend')

from app.core.database import get_db, init_db
from app.models.models import User
from sqlalchemy import select, delete

async def reset_admin():
    await init_db()
    async for db in get_db():
        # Delete all users
        await db.execute(delete(User))
        await db.commit()
        
        # Create admin with simple plaintext hash that works
        # Using werkzeug-style password hash instead of bcrypt
        admin = User(
            username='admin',
            email='admin@local',
            # This is a simple hash, we'll create it differently
            hashed_password='plaintext_admin',  # Temporary
            is_admin=True,
            is_active=True
        )
        
        # Now hash it properly using passlib without bcrypt
        from passlib.hash import pbkdf2_sha256
        admin.hashed_password = pbkdf2_sha256.hash('admin')
        
        db.add(admin)
        await db.commit()
        await db.refresh(admin)
        
        print('✅ Admin user reset successfully!')
        print('Username: admin')
        print('Password: admin')
        print(f'Hash: {admin.hashed_password[:50]}...')
        break

if __name__ == '__main__':
    asyncio.run(reset_admin())
