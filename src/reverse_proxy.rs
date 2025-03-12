use crate::db::DB;
use crate::runtime::AvailableBackends;
use crate::settings::BackendIdentifier;
use hyper::{Body, Request, Response, StatusCode, Uri};
use rand::seq::SliceRandom;
use std::convert::Infallible;
use std::fmt::format;
use std::net::IpAddr;
use std::ops::Index;
use std::sync::{Arc, Mutex};

fn debug_request(req: Request<Body>) -> Result<Response<Body>, Infallible> {
    let body_str = format!("{:?}", req);
    Ok(Response::new(Body::from(body_str)))
}

fn create_route(
    pad_id: Option<String>,
    available_backends: Arc<Mutex<AvailableBackends>>,
    db: DB,
) -> Option<BackendIdentifier> {
    // If the route isn't for a specific padID IE it's for a static file
    // we can use any of the backends but now let's use the first :)
    if pad_id.is_none() {
        let available_backends = available_backends.lock().unwrap();
        let new_backend = available_backends
            .available
            .choose(&mut rand::thread_rng())
            .unwrap();
        return Some(new_backend.clone());
    }
    let pad_id = pad_id.unwrap();
    let result = db.get(&format!("padId:{}", pad_id));
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
                db.set(&format!("padId:{}", pad_id), new_backend);
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
            db.set(&format!("padId:{}", pad_id), new_backend);
            if available_backends.available.is_empty() {
                log::error!("No available backends");
                return None;
            }
            Some(pad_id)
        }
    }
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

pub async fn handle(client_ip: IpAddr, req: Request<Body>, available_backends:
Arc<Mutex<AvailableBackends>>, db: DB) -> Result<Response<Body>,
    Infallible> {
    let path = req.uri();
    let pad_id = get_pad_id(path);
    let chosen_backend = create_route(pad_id, available_backends.clone(), db);
    match chosen_backend {
        Some(backend) => {
            req.uri_mut() =
            Ok(Response::builder()
                .status(StatusCode::SERVICE_UNAVAILABLE)
                .body(Body::empty())?)
        },
        None => Ok(Response::builder()
            .status(StatusCode::SERVICE_UNAVAILABLE)
            .body(Body::empty())?),
    }
}
