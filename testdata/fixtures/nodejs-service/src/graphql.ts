import express from 'express';
import { ApolloServer } from '@apollo/server';

const app = express();
const server = new ApolloServer({ typeDefs: '', resolvers: {} });
app.use('/graphql', (req, res) => {});
