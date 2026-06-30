from fastapi import FastAPI

app = FastAPI()


@app.get("/should-not-count")
def test_route():
    return {"ok": True}
