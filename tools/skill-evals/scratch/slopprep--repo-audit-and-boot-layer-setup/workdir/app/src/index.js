const express = require('express');
const app = express();
const PORT = process.env.PORT || 3000;

app.use(express.json());

const items = [];

app.get('/items', (req, res) => {
  res.json(items);
});

app.post('/items', (req, res) => {
  const item = { id: Date.now(), ...req.body };
  items.push(item);
  res.status(201).json(item);
});

app.listen(PORT, () => {
  console.log(`Server running on port ${PORT}`);
});
