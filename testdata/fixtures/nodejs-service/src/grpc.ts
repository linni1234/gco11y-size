import grpc from '@grpc/grpc-js';

const server = new grpc.Server();
server.addService(checkout.Greeter.service, {
  SayHello: (call, callback) => {
    callback(null, {});
  }
});

const client = new checkout.Inventory('inventory-grpc.internal:50051', grpc.credentials.createInsecure());
