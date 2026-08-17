#!/usr/bin/env node
import { readFileSync, writeFileSync } from 'fs';
import { parseCsv } from './parser';

const [,, inputFile, outputFile] = process.argv;

if (!inputFile) {
  console.error('Usage: csv2json <input.csv> [output.json]');
  process.exit(1);
}

const csv = readFileSync(inputFile, 'utf-8');
const result = parseCsv(csv);

if (outputFile) {
  writeFileSync(outputFile, JSON.stringify(result, null, 2));
} else {
  console.log(JSON.stringify(result, null, 2));
}
