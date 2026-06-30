from django.urls import path, re_path
from rest_framework.routers import DefaultRouter

from . import views

router = DefaultRouter()
router.register(r"customers", views.CustomerViewSet, basename="customer")

urlpatterns = [
    path("django/orders/<uuid:order_id>/", views.order_detail),
    re_path(r"^django/reports/(?P<year>[0-9]+)/$", views.report),
]
