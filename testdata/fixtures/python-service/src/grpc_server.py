import grpc

from checkout_pb2_grpc import GreeterServicer, add_GreeterServicer_to_server


class Greeter(GreeterServicer):
    def SayHello(self, request, context):
        return None


server = grpc.server(None)
add_GreeterServicer_to_server(Greeter(), server)
channel = grpc.insecure_channel("inventory-grpc.internal:50051")
