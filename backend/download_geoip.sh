#!/bin/bash

# Download GeoLite2-City database if it doesn't exist
GEOIP_DIR="/app/geoip"
GEOIP_FILE="$GEOIP_DIR/GeoLite2-City.mmdb"

if [ ! -f "$GEOIP_FILE" ]; then
    echo "GeoIP database not found. Downloading..."
    mkdir -p "$GEOIP_DIR"
    
    # Try DB-IP free database (updates monthly)
    # Format: dbip-city-lite-YYYY-MM.mmdb.gz
    CURRENT_MONTH=$(date +%Y-%m)
    
    echo "Trying to download DB-IP database for $CURRENT_MONTH..."
    if wget -q -O /tmp/dbip-city-lite.mmdb.gz "https://download.db-ip.com/free/dbip-city-lite-$CURRENT_MONTH.mmdb.gz" 2>/dev/null; then
        echo "Download successful, extracting..."
        gunzip /tmp/dbip-city-lite.mmdb.gz
        mv /tmp/dbip-city-lite.mmdb "$GEOIP_FILE"
        echo "GeoIP database downloaded successfully"
    else
        # Try previous month
        PREVIOUS_MONTH=$(date -d "1 month ago" +%Y-%m 2>/dev/null || date -v-1m +%Y-%m 2>/dev/null || echo "2024-11")
        echo "Current month failed, trying $PREVIOUS_MONTH..."
        if wget -q -O /tmp/dbip-city-lite.mmdb.gz "https://download.db-ip.com/free/dbip-city-lite-$PREVIOUS_MONTH.mmdb.gz" 2>/dev/null; then
            echo "Download successful, extracting..."
            gunzip /tmp/dbip-city-lite.mmdb.gz
            mv /tmp/dbip-city-lite.mmdb "$GEOIP_FILE"
            echo "GeoIP database downloaded successfully"
        else
            # Try a known working month as fallback
            echo "Previous month failed, trying fallback (2024-10)..."
            if wget -O /tmp/dbip-city-lite.mmdb.gz "https://download.db-ip.com/free/dbip-city-lite-2024-10.mmdb.gz"; then
                gunzip /tmp/dbip-city-lite.mmdb.gz
                mv /tmp/dbip-city-lite.mmdb "$GEOIP_FILE"
                echo "GeoIP database downloaded successfully (fallback)"
            else
                echo "WARNING: Could not download GeoIP database. Geolocation will not be available."
                echo "The application will still work but without location data for traceroutes."
                # Create empty file to prevent repeated download attempts
                touch "$GEOIP_FILE.unavailable"
            fi
        fi
    fi
else
    echo "GeoIP database already exists"
fi
