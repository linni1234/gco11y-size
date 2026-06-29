import { Kafka } from 'kafkajs';
import amqp from 'amqplib';
import { SQSClient, SendMessageCommand, ReceiveMessageCommand } from '@aws-sdk/client-sqs';
import { ServiceBusClient } from '@azure/service-bus';
import { PubSub } from '@google-cloud/pubsub';
import { Queue, Worker } from 'bullmq';

const kafka = new Kafka({ clientId: 'checkout' });
const consumer = kafka.consumer({ groupId: 'checkout' });
await consumer.subscribe({ topic: 'orders.created' });
const producer = kafka.producer();
await producer.send({ topic: 'orders.audit', messages: [] });

const channel = await amqp.connect('amqp://rabbit.internal').then(conn => conn.createChannel());
channel.consume('orders.queue', () => {});
channel.publish('events', 'order.created', Buffer.from('{}'));

const sqs = new SQSClient({});
await sqs.send(new SendMessageCommand({ QueueUrl: 'https://sqs.us-east-1.amazonaws.com/123/orders-queue' }));
await sqs.send(new ReceiveMessageCommand({ QueueUrl: 'https://sqs.us-east-1.amazonaws.com/123/orders-work' }));

const serviceBus = new ServiceBusClient('Endpoint=sb://bus.servicebus.windows.net/');
serviceBus.createSender('shipping.events');
serviceBus.createReceiver('billing.commands');

const pubsub = new PubSub();
pubsub.topic('inventory.updated');
pubsub.subscription('billing.created');

const queue = new Queue('email-jobs');
new Worker('email-jobs', async job => {});
await queue.add('email-jobs', {});
