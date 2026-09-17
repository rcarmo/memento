use memento_embed::{
    handle_engine_request, read_request, write_frame, ResponseFrame, ResponseHeader,
};
use memento_gte::Model;
use std::env;
use std::io::{self, BufReader, BufWriter};

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let mut args = env::args().skip(1);
    let model_path = args
        .next()
        .ok_or("usage: memento-embed MODEL [cpu|vulkan|auto] [DEVICE]")?;
    let mode = args
        .next()
        .unwrap_or_else(|| "cpu".into())
        .parse::<memento_embed::backend::Backend>()
        .map_err(io::Error::other)?;
    let selector = args.next();
    if args.next().is_some() {
        return Err("too many arguments".into());
    }
    let model = Model::from_path(model_path)?;
    let mut engine = memento_embed::backend::Engine::new(model, mode, selector.as_deref())?;
    let stdin = io::stdin();
    let stdout = io::stdout();
    let mut reader = BufReader::new(stdin.lock());
    let mut writer = BufWriter::new(stdout.lock());
    loop {
        match read_request(&mut reader) {
            Ok(request) => match handle_engine_request(&mut engine, request) {
                Ok(frame) => write_frame(&mut writer, &frame)?,
                Err(err) => {
                    let frame = ResponseFrame {
                        header: ResponseHeader {
                            id: None,
                            ok: false,
                            method: "error".into(),
                            dimensions: None,
                            count: None,
                            payload_len: 0,
                            error: Some(err.to_string()),
                            backend: Some(engine.info.clone()),
                        },
                        payload: vec![],
                    };
                    write_frame(&mut writer, &frame)?;
                }
            },
            Err(memento_embed::ProtocolError::Io(ref ioerr))
                if ioerr.kind() == io::ErrorKind::UnexpectedEof =>
            {
                break
            }
            Err(err) => {
                let frame = ResponseFrame {
                    header: ResponseHeader {
                        id: None,
                        ok: false,
                        method: "error".into(),
                        dimensions: None,
                        count: None,
                        payload_len: 0,
                        error: Some(err.to_string()),
                        backend: Some(engine.info.clone()),
                    },
                    payload: vec![],
                };
                write_frame(&mut writer, &frame)?;
                break;
            }
        }
    }
    Ok(())
}
