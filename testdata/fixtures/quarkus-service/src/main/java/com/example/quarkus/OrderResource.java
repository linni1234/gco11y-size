package com.example.quarkus;

import jakarta.ws.rs.GET;
import jakarta.ws.rs.POST;
import jakarta.ws.rs.Path;

@Path("/orders")
public class OrderResource {
    @GET
    @Path("/{id}")
    public String getOrder() {
        return "ok";
    }

    @POST
    public String createOrder() {
        return "ok";
    }
}
