// Generate an isolated smoke-test configuration without printing its secret.
import { readFileSync, writeFileSync, chmodSync } from 'node:fs';
import { resolve } from 'node:path';
const filename=process.argv[2];
const config=JSON.parse(readFileSync(filename,'utf8'));
config.device_listen='127.0.0.1:18787';
config.preview_listen='127.0.0.1:18788';
config.data_directory=resolve(filename,'..','runtime');
writeFileSync(filename,JSON.stringify(config,null,2));chmodSync(filename,0o600);
