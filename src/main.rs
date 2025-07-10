use std::{error::Error, fs::{create_dir_all, File}, io::{Read, Write}, path::Path, process::Command};
use reqwest::StatusCode;
use thirtyfour::{prelude::*};

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error + Send + Sync>> {
    let mut caps = DesiredCapabilities::chrome();
    caps.set_headless()?;

    setup_chromedriver().await?;

    let driver = WebDriver::new("http://localhost:9515", caps).await.expect("Couldn't connect to chromedriver. Are you sure it is running?\n");

    println!("--- 7TV Emote Downloader ---
This program currently only supports downloading emotes from a user.");

    selector(&driver).await?;

    driver.quit().await?;
    Ok(())
}

async fn setup_chromedriver() -> Result<(), Box<dyn Error + Send + Sync>> {
    // check txt file
    let config_dir = "data";
    create_dir_all(Path::new(&config_dir))?;

    let config_path = format!("{config_dir}/chromedriver_path.txt");
    let mut driver_path = String::new();

    if !Path::new(&config_path).exists() {
        println!("Make sure chromium or google chrome are installed!
Please specify the chromedriver path: ");
        let stdin = std::io::stdin();
        stdin.read_line(&mut driver_path)?;

        let mut config_file = File::create(&config_path)?;
        config_file.write_all(&driver_path.trim().as_bytes())?;

        println!("Written to file {driver_path}");
    } else {
        let mut config_file = File::open(&config_path)?;
        config_file.read_to_string(&mut driver_path)?;
    }
    
    // run chromedriver
    Command::new(driver_path.trim())
    .arg("--port=9515")
    .spawn().expect("Failed to start chromedriver. Is the path correct? Did you install chromium or google chrome?");

    Ok(())
}

async fn selector(driver: &WebDriver) -> Result<(), Box<dyn Error + Send + Sync>> {
    println!("Please input the user id (you can find that in the url of the user https://.../users/[ID]):");
    let mut user_id = String::new();
    let stdin = std::io::stdin();
    stdin.read_line(&mut user_id)?;

    get_emotes_of_user(&driver, user_id).await?;

    println!("Do you want to download from another user? (y/n):");
    let mut answer = String::new();
    stdin.read_line(&mut answer)?;
    
    if answer.trim().eq("y") || answer.trim().eq("yes") {
        Box::pin(selector(&driver)).await?;
    } else {
        println!("goodbye :3");
    }
    Ok(())
}

async fn get_emotes_of_user(driver: &WebDriver, user_id: String) -> Result<(), Box<dyn Error + Send + Sync>> {
    let user_id = user_id;

    // load user page
    let user_url = std::format!("https://7tv.app/users/{}", user_id.trim());
    driver.goto(user_url).await?;
    println!("Loading page...");

    // wait until emote container is loaded or user not exists
    driver.query(By::ClassName("emotes")).first().await?;

    // query username
    let user_name = driver.query(By::ClassName("name")).without_text("").and_displayed().first().await?.text().await?;
    println!("Found user: {}", &user_name);
    
    // load all emotes by scrolling
    println!("Loading emotes...");
    let mut old_part_emotes_length = 0;
    loop {
        // check if user was not found
        match driver.query(By::ClassName("troll")).nowait().first().await {
            Ok(_troll) => {
                println!("User was not found. Exiting...");
                return Ok(());
            },
            Err(_) => ()
        };
        // get partial emotes
        let part_emotes = driver.query(By::ClassName("emote")).all_from_selector().await?;

        // move to last emote of partial emotes
        if old_part_emotes_length != part_emotes.len() {
            driver.action_chain()
                .move_to_element_center( &part_emotes[part_emotes.len() - 1])
                .perform().await?;

            println!("Loaded {} emotes...", part_emotes.len());
        }

        old_part_emotes_length = part_emotes.len();

        // if "no more emotes" or "no emotes" text NOT found -> repeat
        match driver.query(By::Tag("p"))
        .and_displayed().with_text("No more emotes")
        .or(By::Tag("p")).and_displayed().with_text("No emotes").nowait()
        .first().await {
            Ok(element) => {
                 if element.text().await? == "No emotes" {
                    println!("User has no emotes. Exiting...");
                 }
                 break;
            },
            Err(_) => continue
        };
    }

    let emotes = driver.query(By::ClassName("emote")).all_from_selector().await?;

    if emotes.len() == 0 {
        return Ok(());
    }
    println!("Finished loading {} emotes.", emotes.len());

    // create download path
    let path_name = format!("useremotes/{}", &user_name);
    let download_path = Path::new(&path_name);
    create_dir_all(&download_path)?;

    let mut downloaded_counter = 0;
    // download emotes
    for i in 0..emotes.len() {
        let emote = emotes[i].clone();
        // get url from source
        let sources = emote.find_all(By::Tag("source")).await?;
        // get last source (png or gif depending on emote)
        let source = sources[sources.len() - 1].attr("srcset").await?.unwrap();
        // get first of srcset and fix size
        let url = source.split(" ").next().unwrap().replace("1x", "4x");
        let extension = &url.clone()[url.len() - 3..url.len()];

        // get emote name
        let name = emote.find(By::ClassName("name")).await?.text().await?;
        
        let file_name = format!("{path_name}/{name}.{extension}");
        let file_path = Path::new(&file_name);
        
        // check if file exists
        if file_path.exists() {
            println!("{name}.{extension} already exists - skipping. ({}/{})", i+1, emotes.len());
            continue;
        }

        // write file
        let response = reqwest::get(url.clone()).await?;
        if response.status().eq(&StatusCode::OK) {
            println!("Downloaded: {} {} ({}/{})", &name, &url, i+1,emotes.len());
            downloaded_counter += 1;
        } else {
            println!("Failed! (Error: {}) while downloading: {} {} ({}/{})", response.status().as_str(), &name, &url, i+1,emotes.len());
            continue;
        }

        let mut emote_file = File::create(file_path)?;
        emote_file.write_all(&response.bytes().await?)?;
    }

    println!("Successfully downloaded {} emotes.", downloaded_counter);

    Ok(())
}