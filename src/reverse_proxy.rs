use std::collections::HashMap;
use crate::db::DB;
use crate::runtime::AvailableBackends;
use crate::settings::{BackendIdentifier, Setting};
use rand::seq::SliceRandom;
use std::convert::Infallible;
use std::fmt::format;
use std::net::IpAddr;
use std::ops::Index;
use std::sync::{Arc, LazyLock, Mutex};
use axum::body::Body;
use axum::extract::{Request, State};
use axum::http::{StatusCode, Uri};
use axum::response::{IntoResponse, Response};
use crate::Client;
use regex::Regex;

fn debug_request(req: Request<Body>) -> Result<Response<Body>, Infallible> {
    let body_str = format!("{:?}", req);
    Ok(Response::new(Body::from(body_str)))
}

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
            let mut available_backends = available_backends.lock().unwrap();
            if available_backends.available.is_empty() {
                log::error!("No available backends");
                return None;
            }

            if available_backends
                .up
                .iter()
                .position(|e| e == &backend_id)
                .is_some()
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
                let mut locked_db = db.lock().unwrap();
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

static RESOURCES: LazyLock<HashMap<String, String>> = LazyLock::new(HashMap::new);

static PADINDEX_REGEX: LazyLock<Regex> = LazyLock::new(||Regex::new("^/padbootstrap-[a-zA-Z0-9]+.min.js$").unwrap());


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


mod tests {
    use crate::reverse_proxy::PADINDEX_REGEX;

    #[test]
    fn test_pad_index_regex() {
        let path = "/padbootstrap-KK7I7qP9I3E.min.js";
        assert!(PADINDEX_REGEX.is_match(path));
    }
}
