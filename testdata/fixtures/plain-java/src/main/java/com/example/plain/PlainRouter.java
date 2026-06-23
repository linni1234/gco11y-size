package com.example.plain;

import com.sun.net.httpserver.HttpServer;

public class PlainRouter {
    public void register(HttpServer server, JavalinLike app, CustomRouter router) {
        server.createContext("/health", exchange -> {});
        app.get("/items/{id}", ctx -> {});
        app.post("/items", ctx -> {});
        router.route("DELETE", "/items/{id}", this::deleteItem);
    }

    private void deleteItem() {
    }

    interface JavalinLike {
        void get(String path, Object handler);
        void post(String path, Object handler);
    }

    interface CustomRouter {
        void route(String method, String path, Object handler);
    }
}
