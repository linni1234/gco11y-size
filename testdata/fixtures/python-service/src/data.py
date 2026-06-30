import boto3
import httpx
import redis
import requests
from openai import OpenAI
from pymongo import MongoClient
from qdrant_client import QdrantClient
from sqlalchemy import create_engine

engine = create_engine("postgresql://orders:secret@orders-db.internal:5432/orders")
cache = redis.Redis.from_url("redis://redis.internal:6379/0")
mongo = MongoClient("mongodb://mongo.internal:27017/orders")
requests.get("https://inventory-service.internal/api/items")
httpx.post("https://payments.internal/pay")

dynamodb = boto3.resource("dynamodb")
orders_table = dynamodb.Table("orders-table")

vectors = QdrantClient(collection="orders-vectors")
client = OpenAI(base_url="https://api.openai.com/v1")
