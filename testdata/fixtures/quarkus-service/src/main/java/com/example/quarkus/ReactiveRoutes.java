package com.example.quarkus;

import io.quarkus.vertx.web.Route;
import io.quarkus.vertx.web.RouteBase;
import io.vertx.ext.web.RoutingContext;

@RouteBase(path = "reactive")
public class ReactiveRoutes {
    @Route(path = "ping", methods = Route.HttpMethod.GET)
    String ping() {
        return "pong";
    }

    @Route(path = "/hello/:name", methods = {Route.HttpMethod.GET, Route.HttpMethod.POST})
    @Route(path = "/status")
    public String hello() {
        return "hello";
    }

    @Route(methods = Route.HttpMethod.DELETE)
    public void cleanup(RoutingContext rc) {
        rc.response().end();
    }

    @Route(regex = "/regex/.*")
    public void regexOnly(RoutingContext rc) {
        rc.response().end();
    }
}
