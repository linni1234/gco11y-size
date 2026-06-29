var builder = DistributedApplication.CreateBuilder(args);

var postgres = builder.AddPostgres("orders-db");
var redis = builder.AddRedis("cache");
var rabbit = builder.AddRabbitMQ("messaging");
var kafka = builder.AddKafka("events");

builder.AddProject<Projects.DotnetService>("dotnet-checkout")
    .WithReference(postgres)
    .WithReference(redis)
    .WithReference(rabbit)
    .WithReference(kafka);

builder.Build().Run();
