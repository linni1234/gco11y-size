class OrderModel:
    def __init__(self, order_id: str):
        self.order_id = order_id


def normalize_order(order):
    return {"id": order.order_id}
