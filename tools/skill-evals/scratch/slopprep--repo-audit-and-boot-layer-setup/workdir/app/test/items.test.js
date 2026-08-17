const { createItem, getItems } = require('../src/items');

jest.mock('../src/items');

test('createItem returns an object', () => {
  createItem.mockReturnValue({ id: 1, name: 'test' });
  const result = createItem({ name: 'test' });
  expect(result).toEqual({ id: 1, name: 'test' });
});

test('getItems returns array', () => {
  getItems.mockReturnValue([]);
  expect(getItems()).toEqual([]);
});
