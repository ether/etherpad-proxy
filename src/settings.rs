use std::collections::HashMap;
use std::fs;
use serde::{Deserialize, Serialize};
use crate::error::CustomResult;
use crate::stat::Stat;

pub type BackendIdentifier = String;

#[derive(Deserialize, Serialize, Clone)]
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

impl From<Backend> for String {
    fn from(val: Backend) -> Self {
        format!("http://{}:{}/stats", val.host, val.port)
    }
}

impl Backend {
    pub async fn get_stat(&self) -> Result<Stat, Box<dyn std::error::Error>> {
        let response = reqwest::get(<Backend as Into<String>>::into(self.clone())).await?;
        response.json::<Stat>().await.map_err(|e| e.into())
    }
    pub fn get_stat_sync(&self) -> CustomResult<Stat> {
        let response = reqwest::blocking::get(<Backend as Into<String>>::into(self.clone()))?;
        response.json::<Stat>().map_err(|e| e.into())
    }
}

#[derive(Deserialize, Serialize, Clone)]
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