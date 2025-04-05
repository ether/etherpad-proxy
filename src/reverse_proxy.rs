use crate::db::DB;
use crate::runtime::AvailableBackends;
use crate::settings::{BackendIdentifier, Setting};
use rand::seq::SliceRandom;
use std::sync::{Arc, Mutex};
use axum::extract::{Request, State};
use axum::http::{StatusCode, Uri};
use axum::response::{IntoResponse, Response};
use crate::Client;

fn create_route(
    pad_id: Option<String>,
    available_backends: Arc<Mutex<AvailableBackends>>,
    db: Arc<Mutex<DB>>,
) -> Option<BackendIdentifier> {
    // If the route isn't for a specific padID IE it's for a static file
    // we can use any of the backends but now let's use the first :)
    if pad_id.is_none() {
        let available_backends = available_backends.lock().unwrap();
        let new_backend = available_backends
            .available
            .choose(&mut rand::thread_rng()).cloned();
        return new_backend;
    }
    let pad_id = pad_id.unwrap();
    let result = {
        let locked_db = db.lock().unwrap();
        locked_db.get(&format!("padId:{}", pad_id))
    };
    match result {
        Some(backend_id) => {
            let available_backends = available_backends.lock().unwrap();
            if available_backends.available.is_empty() {
                log::error!("No available backends");
                return None;
            }

            if available_backends
                .up
                .iter()
                .any(|e| e == &backend_id)
            {
                Some(backend_id)
            } else {
                let new_backend = available_backends
                    .up
                    .choose(&mut rand::thread_rng())
                    .unwrap();
                {
                    let locked_db = db.lock().unwrap();
                    locked_db.set(&format!("padId:{}", pad_id), new_backend);
                }
                log::info!(
                    "Creating new association for pad {} with backend {}",
                    pad_id,
                    new_backend
                );
                Some(new_backend.clone())
            }
        }
        None => {
            let available_backends = available_backends.lock().unwrap();
            let new_backend = available_backends
                .available
                .choose(&mut rand::thread_rng())
                .unwrap();
            {
                let locked_db = db.lock().unwrap();
                locked_db.set(&format!("padId:{}", pad_id), new_backend);
            }
            log::info!("Creating new association for pad {} with backend {}", pad_id, new_backend);
            if available_backends.available.is_empty() {
                log::error!("No available backends");
                return None;
            }
            Some(new_backend.clone())
        }
    }
}

#[derive(Clone)]
pub struct StateOfReverseProxy {
    pub client: Client,
    pub available_backends: Arc<Mutex<AvailableBackends>>,
    pub db: Arc<Mutex<DB>>,
    pub setting: Setting,
}


pub async fn handler(State(client): State<StateOfReverseProxy>, mut req: Request) ->
                                                                                  Result<Response,
    StatusCode> {
    let path = req.uri();
    let path_query = req
        .uri()
        .path_and_query()
        .map(|v| v.as_str())
        .unwrap_or(path.path());


    let pad_id = get_pad_id(path);
    let chosen_backend = create_route(pad_id, client.available_backends.clone(), client.db);
    if let Some(backend_id) = chosen_backend {
        let backend = client.setting.backends.get(&backend_id).cloned().unwrap();
        let uri = format!("http://{}:{}{}",backend.host, backend.port, path_query);
        *req.uri_mut() = Uri::try_from(uri).unwrap();
    } else {
        return Err(StatusCode::BAD_REQUEST);
    }


    let response = client
        .client
        .request(req)
        .await
        .map_err(|_| StatusCode::BAD_REQUEST)?
        .into_response();

    Ok(response)
}

pub fn get_pad_id(uri: &Uri) -> Option<String> {
    let mut pad_id: Option<String> = None;
    let path = uri.path();
    if path.contains("/p/") {
        let split: Vec<_> = path.split("/p/").collect();
        let quest_mark: Vec<_> = split[1].split("?").collect();
        let final_result: Vec<_> = quest_mark[0].split("/").collect();
        pad_id = Some(final_result[0].to_string());
    }

    if pad_id.is_none() {
        if let Some(query) = uri.query() {
            let query: Vec<_> = query.split("&").collect();
            for q in query {
                let q: Vec<_> = q.split("=").collect();
                if q[0] == "padId" {
                    pad_id = Some(q[1].to_string());
                }
            }
        }
    }
    pad_id
}
