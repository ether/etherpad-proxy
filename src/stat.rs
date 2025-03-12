use serde::Deserialize;

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Stat {
    pub active_pads: Option<u32>,
}