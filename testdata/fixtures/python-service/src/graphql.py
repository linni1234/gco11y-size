import strawberry
from fastapi import FastAPI

app = FastAPI()


@strawberry.type
class Query:
    hello: str = "world"


schema = strawberry.Schema(Query)
app.add_api_route("/graphql", lambda request: None, methods=["POST"])
