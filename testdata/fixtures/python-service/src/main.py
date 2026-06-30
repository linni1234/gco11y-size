from fastapi import APIRouter, FastAPI, WebSocket
from starlette.routing import Route, WebSocketRoute

app = FastAPI(root_path="/api")
router = APIRouter(prefix="/orders")


@router.get("/{order_id}")
async def get_order(order_id: str):
    return {"id": order_id}


@router.post("")
async def create_order():
    return {"ok": True}


@router.websocket("/stream/{client_id}")
async def order_stream(websocket: WebSocket, client_id: str):
    await websocket.accept()


def reports():
    return {"ok": True}


def ping(request):
    return None


def ws(websocket):
    return None


app.include_router(router, prefix="/v1")
app.add_api_route("/reports", reports, methods=["POST"])

routes = [
    Route("/starlette/ping", ping),
    WebSocketRoute("/starlette/ws", ws),
]
