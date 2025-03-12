use std::error;

pub type CustomResult<T> = Result<T, Box<dyn error::Error>>;
