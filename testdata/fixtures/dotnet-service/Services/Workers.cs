using System.Diagnostics;
using Orleans;
using Orleans.Streams;

namespace DotnetCheckout.Services;

public class OrderWorker : BackgroundService
{
    private static readonly ActivitySource ActivitySource = new("dotnet-checkout");

    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        using var activity = ActivitySource.StartActivity("reconcile-orders");
        activity?.SetTag("user.id", "42");
        await Task.Delay(1, stoppingToken);
    }
}

public class OrderGrain : Grain, IGrainWithStringKey
{
    public async Task Subscribe(IAsyncStream<string> stream)
    {
        await stream.SubscribeAsync((value, token) => Task.CompletedTask);
        await stream.OnNextAsync("created");
    }
}
