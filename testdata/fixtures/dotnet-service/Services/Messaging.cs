using Azure.Messaging.ServiceBus;
using Confluent.Kafka;
using MassTransit;
using NServiceBus;
using RabbitMQ.Client;

namespace DotnetCheckout.Services;

public class Messaging
{
    public void Configure(IConsumer<string, string> consumer, IProducer<string, string> producer, IModel channel, ServiceBusClient serviceBus, IBusRegistrationConfigurator bus)
    {
        consumer.Subscribe("orders.created");
        producer.ProduceAsync("orders.audit", new Message<string, string>());
        channel.BasicConsume(queue: "orders.queue", autoAck: false, consumer: null);
        channel.BasicPublish(exchange: "events", routingKey: "order.created", basicProperties: null, body: default);
        serviceBus.CreateProcessor("billing.commands");
        serviceBus.CreateSender("shipping.events");
        bus.ReceiveEndpoint("orders-masstransit", endpoint => { });
    }

    public void ConfigureEndpoint(EndpointConfiguration endpointConfiguration)
    {
        endpointConfiguration.EndpointName("orders-endpoint");
    }
}
