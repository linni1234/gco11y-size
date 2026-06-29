using System.Data.SqlClient;
using Dapper;
using Microsoft.Azure.Cosmos;
using MongoDB.Driver;
using Npgsql;
using StackExchange.Redis;

namespace DotnetCheckout.Services;

public class OrdersDbContext : DbContext
{
    public DbSet<Order> Orders => Set<Order>();
}

public class Order
{
    public string Id { get; set; } = "";
}

public class DataClients
{
    public async Task Load()
    {
        using var sql = new SqlConnection("Server=sql.internal;Database=orders;User Id=app;Password=secret");
        await sql.QueryAsync<Order>("select * from orders");

        using var pg = new NpgsqlConnection("Host=postgres.internal;Database=readmodel");
        await pg.ExecuteAsync("select 1");

        var redis = ConnectionMultiplexer.Connect("redis.internal:6379");
        var mongo = new MongoClient("mongodb://mongo.internal/orders");
        var cosmos = new CosmosClient("AccountEndpoint=https://cosmos.internal:443/;AccountKey=fake");
        var http = new HttpClient { BaseAddress = new Uri("https://inventory-service.internal/api/") };
        var grpc = Grpc.Net.Client.GrpcChannel.ForAddress("https://inventory-grpc.internal:5001");
    }
}
