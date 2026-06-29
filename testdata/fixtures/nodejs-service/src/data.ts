import { PrismaClient } from '@prisma/client';
import { DataSource } from 'typeorm';
import { Sequelize } from 'sequelize';
import mongoose from 'mongoose';
import { Pool } from 'pg';
import { createClient } from 'redis';

const prisma = new PrismaClient();
const dataSource = new DataSource({
  type: 'postgres',
  database: 'orders'
});
const sequelize = new Sequelize('mysql://mysql.internal/shop');
await mongoose.connect('mongodb://mongo.internal/orders');
const pool = new Pool({ connectionString: 'postgres://postgres.internal/readmodel' });
const redis = createClient({ url: 'redis://redis.internal:6379' });
