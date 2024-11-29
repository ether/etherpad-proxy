import {GenericContainer, type PortWithOptionalBinding, type StartedTestContainer} from "testcontainers";
import {afterAll, beforeAll, describe, it} from "vitest";
import {start, close} from '../runtime.ts'
import type {BindMount} from "testcontainers/build/types";
import {execSync} from 'node:child_process'
import path from 'node:path'
import * as os from "node:os";

describe('etherpad proxy test', ()=>{
    let containers: StartedTestContainer[] = []

    beforeAll(async () => {
        for (let i = 0 ; i< 3; i++) {
            const portMappings: PortWithOptionalBinding[] = [
                { container: 9001, host: 9001 + i },
            ];
            let promises: Promise<StartedTestContainer>[] = []
            let currentDirectory = "";
            const isWindows = os.platform() === 'win32';

            if (isWindows) {
                console.log(path.resolve())
                currentDirectory = path.resolve()
                //currentDirectory = execSync(`wsl /bin/wslpath ${currentDirectory}`).toString().trim()
            } else {
                currentDirectory = path.resolve()
            }

            const mount = {
                mode: 'rw',
                source: path.join(currentDirectory, 'test' + i+1),
                target:  '/opt/etherpad-lite/var',
            } satisfies BindMount
            console.log("Mounts are", mount)

            promises.push(new GenericContainer("etherpad/etherpad:develop")
                .withName("etherpad_"+(i+1))
                .withBindMounts([mount])
                .withExposedPorts(...portMappings)
                .start())

            containers = await Promise.all(promises)
        }
    })

    it('Etherpad is starting', ()=>{
        const settings = {
            "port": 9000,
            "backends" : {
            "backend1": {
                "host": "localhost",
                    "port": 9001
            }
        },
            "maxPadsPerInstance": 5,
            "checkInterval": 1000,
            "dbType": "dirty",
            "dbSettings": {
            "filename": "dirty.db"
        }
        }

       start(settings)
        try {
            execSync('etherpad-loadtest http://localhost:9000/p/test2 -d 60 &')
        } catch (error: any) {
            console.error(`Error: ${error.message}`);
            console.error(`Exit code: ${error.status}`);
        }

    })


    afterAll(async () => {
        close()
        for (const container of containers) {
            await container.stop()
        }
    })
})
