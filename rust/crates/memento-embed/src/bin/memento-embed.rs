use memento_embed::{handle_request, read_request, write_frame, ResponseFrame, ResponseHeader};
use memento_gte::Model;
use std::env;
use std::io::{self, BufReader, BufWriter};

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let model_path = env::args()
        .nth(1)
        .ok_or("usage: memento-embed <model.gtemodel>")?;
    let model = Model::from_path(model_path)?;
    let stdin = io::stdin();
    let stdout = io::stdout();
    let mut reader = BufReader::new(stdin.lock());
    let mut writer = BufWriter::new(stdout.lock());
    loop {
        match read_request(&mut reader) {
            Ok(request) => match handle_request(&model, request) {
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
                    },
                    payload: vec![],
                };
                write_frame(&mut writer, &frame)?;
            }
        }
    }
    Ok(())
}
