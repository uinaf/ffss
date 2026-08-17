import { parseCsv } from './parser';

// No jest.mock here on purpose. Mocking the module under test made this file
// assert only that an import existed, so it stayed green through broken parses.

describe('parseCsv', () => {
  test('maps each row onto the header names', () => {
    expect(parseCsv('id,name\n1,Ada\n2,Grace')).toEqual([
      { id: '1', name: 'Ada' },
      { id: '2', name: 'Grace' },
    ]);
  });

  test('trims surrounding whitespace from headers and values', () => {
    expect(parseCsv(' id , name \n 1 , Ada ')).toEqual([{ id: '1', name: 'Ada' }]);
  });

  test('fills missing trailing columns with an empty string', () => {
    expect(parseCsv('id,name,role\n1,Ada')).toEqual([{ id: '1', name: 'Ada', role: '' }]);
  });

  test('ignores leading and trailing blank lines', () => {
    expect(parseCsv('\nid,name\n1,Ada\n')).toEqual([{ id: '1', name: 'Ada' }]);
  });

  test('returns no rows when only a header is present', () => {
    expect(parseCsv('id,name')).toEqual([]);
  });
});
