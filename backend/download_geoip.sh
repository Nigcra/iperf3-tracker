#!/bin/bash

# Download GeoLite2-City database if it doesn't exist
GEOIP_DIR="/app/geoip"
GEOIP_FILE="$GEOIP_DIR/GeoLite2-City.mmdb"

if [ ! -f "$GEOIP_FILE" ]; then
    echo "GeoIP database not found. Downloading..."
    mkdir -p "$GEOIP_DIR"
    
    # Download from MaxMind (free version)
    # Note: This is a direct download link that may change. 
    # For production, consider using MaxMind's API with a license key
    
    # Alternative: Download from dbip.com (no registration required)
    wget -O /tmp/dbip-city-lite.mmdb.gz "https://download.db-ip.com/free/dbip-city-lite-2024-11.mmdb.gz" && \
    gunzip /tmp/dbip-city-lite.mmdb.gz && \
    mv /tmp/dbip-city-lite.mmdb "$GEOIP_FILE" && \
    echo "GeoIP database downloaded successfully"
else
    echo "GeoIP database already exists"
fi
