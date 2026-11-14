import { render } from 'preact';

import { App } from './app';
import { readConfig } from './config';
import './styles.css';

render(<App config={readConfig()} />, document.getElementById('app')!);
