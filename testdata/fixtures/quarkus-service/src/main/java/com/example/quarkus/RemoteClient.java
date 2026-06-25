package com.example.quarkus;

import org.eclipse.microprofile.rest.client.inject.RegisterRestClient;

import jakarta.ws.rs.GET;
import jakarta.ws.rs.Path;

@Path("/remote")
@RegisterRestClient(configKey = "remote-api")
public interface RemoteClient {
    @GET
    @Path("/{id}")
    public String getRemote();
}
