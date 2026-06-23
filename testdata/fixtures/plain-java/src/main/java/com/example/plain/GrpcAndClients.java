package com.example.plain;

import io.grpc.ManagedChannelBuilder;
import io.grpc.stub.StreamObserver;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;

public class GrpcAndClients extends GreeterGrpc.GreeterImplBase {
    public void sayHello(HelloRequest request, StreamObserver<HelloReply> responseObserver) {
    }

    public void clients() {
        ManagedChannelBuilder.forAddress("inventory-service", 9090);
        HttpClient.newHttpClient().send(HttpRequest.newBuilder(URI.create("https://payments.internal/pay")).build(), null);
    }
}
