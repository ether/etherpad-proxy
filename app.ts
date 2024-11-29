import fs from 'fs'
import {start} from './runtime'
// load the settings
const loadSettings = () => {
  let settings;
  try {
    const settings_location = process.env["SETTINGS_FILE"] || "settings.json"
    settings = fs.readFileSync(settings_location, 'utf8');
    return JSON.parse(settings);
  } catch (e) {
    console.error('Please create your own settings.json file');
    settings = fs.readFileSync('settings.json.template', 'utf8');
    return JSON.parse(settings);
  }
};

const settings = loadSettings();
start(settings);
