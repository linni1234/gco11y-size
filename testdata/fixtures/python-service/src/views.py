from rest_framework.decorators import action, api_view
from rest_framework.viewsets import ViewSet


@api_view(["GET"])
def order_detail(request, order_id):
    return None


def report(request, year):
    return None


class CustomerViewSet(ViewSet):
    @action(detail=True, methods=["POST"], url_path="sync")
    def sync(self, request, pk=None):
        return None
