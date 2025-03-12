use std::collections::HashMap;
use std::fs;
use serde::{Deserialize, Serialize};
use crate::error::CustomResult;
use crate::stat::Stat;

pub type BackendIdentifier = String;

#[derive(Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct Setting {
    pub port: u16,
    pub backends: HashMap<BackendIdentifier, Backend>,
    pub max_pads_per_instance: u32,
    pub check_interval: u64,
    pub db_type: String,
    pub db_settings: DBSettings
}

#[derive(Deserialize, Serialize, Clone)]
pub struct Backend {
    pub host: String,
    pub port: u16,
}

impl Into<String> for Backend {
    fn into(self) -> String {
        format!("http://{}:{}/stats", self.host, self.port)
    }
}

impl Backend {
    pub fn get_stat(&self) -> Result<Stat, Box<dyn std::error::Error>> {
        let response = reqwest::blocking::get(<Backend as Into<String>>::into(self.clone()))?;
        response.json::<Stat>().map_err(|e| e.into())
    }
}

#[derive(Deserialize, Serialize)]
pub struct DBSettings {
    pub filename: String,
}




impl Setting {
    pub fn try_new() -> anyhow::Result<Setting> {
        let settings_location: String = std::env::var("SETTING_FILE").unwrap_or("settings.json"
            .to_string());
        let string = fs::read_to_string(&settings_location)?;
        let settings: Setting = serde_json::from_str(&string)?;
        Ok(settings)
    }
}