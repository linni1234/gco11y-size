package com.example.plain;

import jakarta.ws.rs.GET;
import jakarta.ws.rs.POST;
import jakarta.ws.rs.Path;

@Path("/jax")
public class JaxRsResource {
    @GET
    @Path("/orders/{id}")
    public String getOrder() {
        return "ok";
    }

    @POST
    @Path("/orders")
    public String createOrder() {
        return "ok";
    }
}
