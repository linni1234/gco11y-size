using Hangfire;
using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.SignalR;
using Microsoft.EntityFrameworkCore;
using OpenTelemetry.Resources;
using Quartz;

var builder = WebApplication.CreateBuilder(args);

builder.Services.AddControllers();
builder.Services.AddSignalR();
builder.Services.AddGrpc();
builder.Services.AddHealthChecks();
builder.Services.AddReverseProxy().LoadFromConfig(builder.Configuration.GetSection("ReverseProxy"));
builder.Services.AddDbContext<OrdersDbContext>(options =>
    options.UseSqlServer(builder.Configuration.GetConnectionString("OrdersDb")));
builder.Services.AddOpenTelemetry().ConfigureResource(resource => resource.AddService("dotnet-checkout"));
builder.Services.AddHostedService<OrderWorker>();
builder.Services.AddQuartz();
builder.Services.AddHangfire(config => config.UseSqlServerStorage(builder.Configuration.GetConnectionString("OrdersDb")));

var app = builder.Build();
var api = app.MapGroup("/api");
var v1 = api.MapGroup("/v1");

v1.MapGet("/orders/{id:int}", (int id) => Results.Ok(id));
v1.MapPost("/orders", () => Results.Created("/api/v1/orders/1", null));
app.MapMethods("/reports/{year:int?}", new[] { "GET", "POST" }, () => Results.Ok());
app.MapHealthChecks("/health");
app.MapHub<OrderHub>("/hubs/orders");
app.MapGrpcService<GreeterService>();
app.MapControllerRoute(name: "default", pattern: "{controller=Home}/{action=Index}/{id?}");
app.MapReverseProxy();

RecurringJob.AddOrUpdate("cleanup", () => Console.WriteLine("cleanup"), Cron.Daily);

app.Run();

public sealed class OrderHub : Hub
{
}
