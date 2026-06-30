import boto3
import pika
from apscheduler.schedulers.background import BackgroundScheduler
from celery import Celery
from google.cloud import pubsub_v1
from kafka import KafkaConsumer, KafkaProducer
from rq import Queue

celery = Celery("checkout", broker="redis://redis.internal/0", backend="redis://redis.internal/1")


@celery.task(queue="orders.created")
def reconcile_order(order_id):
    return order_id


celery.send_task("billing.reprice", queue="billing.commands")

queue = Queue("reports")
queue.enqueue(reconcile_order)

consumer = KafkaConsumer("orders.created")
producer = KafkaProducer(bootstrap_servers="kafka.internal:9092")
producer.send("orders.audit", b"{}")

channel = pika.BlockingConnection().channel()
channel.basic_consume(queue="orders.queue")
channel.basic_publish(exchange="events", routing_key="order.created", body=b"{}")

sqs = boto3.client("sqs")
sqs.send_message(QueueUrl="https://sqs.us-east-1.amazonaws.com/123/orders-queue", MessageBody="{}")

publisher = pubsub_v1.PublisherClient()
publisher.topic_path("proj", "orders-topic")
subscriber = pubsub_v1.SubscriberClient()
subscriber.subscription_path("proj", "orders-subscription")

scheduler = BackgroundScheduler()


@scheduler.scheduled_job("interval", minutes=5)
def refresh_read_model():
    return None
