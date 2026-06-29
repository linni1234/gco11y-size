import { Controller, Get, Post, WebSocketGateway } from '@nestjs/common';

@Controller('nest/orders')
export class OrdersController {
  @Get(':id')
  findOne() {
    return {};
  }

  @Post()
  create() {
    return {};
  }
}

@WebSocketGateway()
export class OrdersGateway {}
