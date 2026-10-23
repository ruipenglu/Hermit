from hermit.probe import connection_chain


class Connections:
    def __init__(self, connections): self.connections = connections
    def request(self, *args): return {"connections": self.connections}


def test_correlates_exact_request_not_other_users():
    def connection(source=50000, inbound=18080, host="api.ipify.org", ip="127.0.0.1", chain=None):
        return {"metadata": {"sourcePort": source, "inboundPort": inbound, "host": host, "sourceIP": ip}, "chains": chain or ["A"]}
    clash = Connections([connection(source=50001, chain=["wrong"]), connection(inbound=18081, chain=["wrong"]),
                         connection(host="example.org", chain=["wrong"]), connection(ip="192.168.1.1", chain=["wrong"]), connection()])
    assert connection_chain(clash, 50000, 18080, "api.ipify.org") == ["A"]
    assert connection_chain(clash, 9999, 18080, "api.ipify.org") == []
