from fastapi import APIRouter
from typing import List
from pydantic import BaseModel

router = APIRouter(prefix="/public-servers", tags=["public-servers"])


class PublicServer(BaseModel):
    """Public iperf3 server information"""
    name: str
    host: str
    port: int
    location: str
    provider: str
    description: str


# Curated list of public iperf3 servers
PUBLIC_SERVERS = [
    PublicServer(
        name="Bouygues Telecom Paris",
        host="iperf.par2.as5410.net",
        port=5200,
        location="Paris, France",
        provider="Bouygues Telecom",
        description="Public iperf3 server in Paris, France"
    ),
    PublicServer(
        name="Bouygues Telecom Proof",
        host="proof.ovh.net",
        port=5201,
        location="France",
        provider="OVH",
        description="OVH public test server"
    ),
    PublicServer(
        name="Online.net Paris",
        host="ping.online.net",
        port=5200,
        location="Paris, France",
        provider="Online.net",
        description="Online.net public iperf3 server"
    ),
    PublicServer(
        name="Worldstream Netherlands",
        host="speedtest.wtnet.de",
        port=5200,
        location="Netherlands",
        provider="Worldstream",
        description="Worldstream public iperf3 server"
    ),
    PublicServer(
        name="Speedtest Frankfurt",
        host="speedtest.fra.de.as9136.net",
        port=5200,
        location="Frankfurt, Germany",
        provider="AS9136",
        description="Public iperf3 server in Frankfurt"
    ),
    PublicServer(
        name="Speedtest USA East",
        host="speedtest.uztelecom.uz",
        port=5200,
        location="USA East Coast",
        provider="Various",
        description="Public iperf3 server on US East Coast"
    ),
    PublicServer(
        name="AT&T Ashburn VA",
        host="speedtest.ashb.va.us.as7018.net",
        port=5201,
        location="Ashburn, Virginia, USA",
        provider="AT&T",
        description="AT&T public iperf3 server in Virginia"
    ),
    PublicServer(
        name="Hurricane Electric",
        host="speedtest.dal.tx.us.he.net",
        port=5201,
        location="Dallas, Texas, USA",
        provider="Hurricane Electric",
        description="Hurricane Electric public server in Dallas"
    ),
    PublicServer(
        name="Vodafone Portugal",
        host="speedtest.vodafone.pt",
        port=5201,
        location="Portugal",
        provider="Vodafone",
        description="Vodafone Portugal public iperf3 server"
    ),
    PublicServer(
        name="Init7 Switzerland",
        host="speedtest.init7.net",
        port=5201,
        location="Switzerland",
        provider="Init7",
        description="Init7 Swiss public iperf3 server"
    ),
]


@router.get("", response_model=List[PublicServer])
async def get_public_servers():
    """Get list of known public iperf3 servers"""
    return PUBLIC_SERVERS


@router.get("/search")
async def search_public_servers(query: str):
    """Search public servers by name, location, or provider"""
    query_lower = query.lower()
    results = [
        server for server in PUBLIC_SERVERS
        if (query_lower in server.name.lower() or
            query_lower in server.location.lower() or
            query_lower in server.provider.lower())
    ]
    return results
