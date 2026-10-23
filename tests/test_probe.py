from hermit.probe import connection_chain
from hermit.probe import probe
import pytest


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


@pytest.mark.parametrize('chain, status', [(['A'],'verified'), (['B'],'mismatch'), ([], 'unconfirmed')])
def test_reachable_ip_does_not_imply_verified_node(monkeypatch, chain, status):
    class Sock:
        def __init__(self): self.data = b'\x05\x00\x05\x00\x00\x01\x00\x00\x00\x00\x00\x00'
        def getsockname(self): return ('127.0.0.1',50000)
        def sendall(self, data): pass
        def close(self): pass
        def recv(self,size):
            result,self.data = self.data[:size],self.data[size:]
            return result
    class TLS:
        def wrap_socket(self,sock,**kwargs): return sock
    class Response:
        status = 200
        def __init__(self,*args): pass
        def begin(self): pass
        def read(self,*args): return b'{"ip":"203.0.113.1"}'
    monkeypatch.setattr('hermit.probe.socket.create_connection',lambda *a,**k:Sock())
    monkeypatch.setattr('hermit.probe.ssl.create_default_context',lambda **kwargs:TLS())
    monkeypatch.setattr('hermit.probe.http.client.HTTPResponse',Response)
    monkeypatch.setattr('hermit.probe.connection_chain',lambda *a:chain)
    result = probe(None,18080,'A')
    assert result['status'] == status and result['exitIp'] == '203.0.113.1'
    assert result['identityConfirmed'] == (status == 'verified')
