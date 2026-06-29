using Microsoft.AspNetCore.Mvc;

namespace DotnetCheckout.Controllers;

[ApiController]
[Route("api/[controller]")]
public class OrdersController : ControllerBase
{
    [HttpGet("{id:int}")]
    public IActionResult GetOrder(string id) => Ok(id);

    [HttpPost("{id:int}/cancel")]
    public IActionResult Cancel(string id) => Accepted();
}
