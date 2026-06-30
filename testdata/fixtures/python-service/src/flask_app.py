from flask import Blueprint, Flask

app = Flask(__name__)
bp = Blueprint("checkout", __name__, url_prefix="/flask")


@bp.route("/orders/<order_id>", methods=["GET"])
def get_flask_order(order_id):
    return {"id": order_id}


@bp.route("/orders", methods=["POST"])
def create_flask_order():
    return {"ok": True}


app.register_blueprint(bp, url_prefix="/api")
